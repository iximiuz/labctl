package kubeproxy

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

const defaultControlPlaneMachine = "cplane-01"

type options struct {
	playID       string
	address      string
	controlPlane string
}

func (o *options) localHost() string {
	parts := strings.Split(o.address, ":")
	if len(parts) == 3 {
		return parts[0]
	}
	return "127.0.0.1"
}

func (o *options) localPort() string {
	parts := strings.Split(o.address, ":")
	if len(parts) == 3 {
		return parts[1]
	}
	return parts[0]
}

func (o *options) remotePort() string {
	parts := strings.Split(o.address, ":")
	return parts[len(parts)-1]
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:               "kube-proxy <playground-id>",
		Short:             "Forward Kubernetes API port and set up kubeconfig for local kubectl access",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]
			return labcli.WrapStatusError(runKubeProxy(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(
		&opts.address,
		"address",
		"127.0.0.1:6443:6443",
		`Local address mapping in the form [LOCAL_HOST:]LOCAL_PORT:REMOTE_PORT`,
	)
	flags.StringVar(
		&opts.controlPlane,
		"control-plane",
		"",
		`Control plane machine name (default: "cplane-01" if present, otherwise the first machine)`,
	)

	return cmd
}

func runKubeProxy(ctx context.Context, cli labcli.CLI, opts *options) error {
	if err := validateAddress(opts.address); err != nil {
		return labcli.NewStatusError(1, "invalid --address %q: %s", opts.address, err)
	}

	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	machine := opts.controlPlane
	if machine == "" {
		if p.GetMachine(defaultControlPlaneMachine) != nil {
			machine = defaultControlPlaneMachine
		} else {
			machine = p.Machines[0].Name
		}
	} else if p.GetMachine(machine) == nil {
		return labcli.NewStatusError(1, "machine %q not found in the playground", machine)
	}

	user, err := p.ResolveUser(machine, "")
	if err != nil {
		return err
	}

	localHost := opts.localHost()
	localPort := opts.localPort()
	remotePort := opts.remotePort()

	kubeconfigPath := filepath.Join(
		cli.Config().PlaysDir,
		opts.playID+"-"+machine+"-"+user,
		"kubeconfig",
	)
	if err := os.MkdirAll(filepath.Dir(kubeconfigPath), 0o700); err != nil {
		return fmt.Errorf("couldn't create kubeconfig directory: %w", err)
	}

	cli.PrintAux("Downloading kubeconfig from %q...\n", machine)
	if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
		PlayID:  opts.playID,
		Machine: machine,
		User:    user,
		Quiet:   true,
		WithProxy: func(ctx context.Context, info *sshproxy.SSHProxyInfo) error {
			cmd := exec.CommandContext(ctx, "scp",
				"-i", info.IdentityFile,
				"-o", "StrictHostKeyChecking=no",
				"-o", "UserKnownHostsFile=/dev/null",
				"-P", info.ProxyPort,
				fmt.Sprintf("%s@%s:~/.kube/config", info.User, info.ProxyHost),
				kubeconfigPath,
			)
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("scp failed: %w\n%s", err, strings.TrimSpace(string(out)))
			}
			return nil
		},
	}); err != nil {
		return fmt.Errorf("couldn't download kubeconfig: %w", err)
	}

	cli.PrintAux("Patching kubeconfig server address to %s:%s...\n", localHost, localPort)
	if err := patchKubeconfigServer(kubeconfigPath, localHost, localPort); err != nil {
		return fmt.Errorf("couldn't patch kubeconfig: %w", err)
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:          p.ID,
		Machine:         machine,
		SSHUser:         user,
		SSHIdentityFile: cli.Config().SSHIdentityFile,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	kubeAPISpec := portforward.ForwardingSpec{
		Kind:       "local",
		LocalHost:  localHost,
		LocalPort:  localPort,
		RemotePort: remotePort,
	}
	cli.PrintAux("Forwarding %s (local) -> :%s (remote)\n", kubeAPISpec.LocalAddr(), remotePort)
	kubeAPIDoneCh := tunnel.StartForwarding(ctx, kubeAPISpec)

	cli.PrintAux("Waiting for the Kubernetes API to become accessible...\n")
	if err := waitForPort(ctx, net.JoinHostPort(localHost, localPort), 30*time.Second); err != nil {
		return fmt.Errorf("Kubernetes API port didn't become available: %w", err)
	}

	cli.PrintOut("\nKubeconfig saved to:\n  %s\n", kubeconfigPath)
	cli.PrintOut("\nTo access the cluster:\n\n")
	cli.PrintOut("  export KUBECONFIG=%s\n", kubeconfigPath)
	cli.PrintOut("  kubectl get all\n")
	cli.PrintOut("\nOr using an explicit flag:\n\n")
	cli.PrintOut("  kubectl --kubeconfig=%s get all\n", kubeconfigPath)
	cli.PrintOut("\nKeeping port forwarding running. Press Ctrl+C to stop.\n\n")

	select {
	case err := <-kubeAPIDoneCh:
		return err
	case <-ctx.Done():
		return nil
	}
}

func validateAddress(addr string) error {
	parts := strings.Split(addr, ":")
	switch len(parts) {
	case 2, 3:
		return nil
	default:
		return fmt.Errorf("expected [LOCAL_HOST:]LOCAL_PORT:REMOTE_PORT")
	}
}

func waitForPort(ctx context.Context, addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 1*time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out after %s waiting for %s", timeout, addr)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(300 * time.Millisecond):
		}
	}
}

func patchKubeconfigServer(kubeconfigPath, localHost, localPort string) error {
	data, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return err
	}

	// Use yaml.Node so the rest of the document (certs, contexts, etc.) is
	// preserved verbatim and only the server values are touched.
	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("invalid kubeconfig: %w", err)
	}

	newServer := "https://" + net.JoinHostPort(localHost, localPort)
	setClusterServers(&doc, newServer)

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	return os.WriteFile(kubeconfigPath, out, 0o600)
}

// setClusterServers walks the YAML node tree and replaces every scalar value
// whose sibling key is "server" and whose parent chain goes through "cluster"
// → "clusters". This is exactly the path kubeconfig uses.
func setClusterServers(n *yaml.Node, server string) {
	if n.Kind == yaml.DocumentNode || n.Kind == yaml.SequenceNode {
		for _, child := range n.Content {
			setClusterServers(child, server)
		}
		return
	}
	if n.Kind != yaml.MappingNode {
		return
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		key, val := n.Content[i], n.Content[i+1]
		if key.Value == "server" && val.Kind == yaml.ScalarNode {
			val.Value = server
		} else {
			setClusterServers(val, server)
		}
	}
}
