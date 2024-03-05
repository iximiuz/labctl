package sshproxy

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

type options struct {
	playID  string
	machine string
	address string
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "ssh-proxy [flags] <playground-id>",
		Short: `Start SSH proxy to the playground's machine`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]

			if opts.address != "" && strings.Count(opts.address, ":") != 1 {
				return fmt.Errorf("invalid address %q", opts.address)
			}

			return labcli.WrapStatusError(runSSHProxy(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		`Target machine (default: the first machine in the playground)`,
	)
	flags.StringVar(
		&opts.address,
		"address",
		"",
		`Local address to map to the machine's SSHD port (default: 'localhost:<random port>')`,
	)

	return cmd
}

func runSSHProxy(ctx context.Context, cli labcli.CLI, opts *options) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine == "" {
		opts.machine = p.Machines[0].Name
	} else {
		if p.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the playground", opts.machine)
		}
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:     opts.playID,
		Machine:    opts.machine,
		SSHDirPath: cli.Config().SSHDirPath,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		localPort = portStr(opts.address)
		localHost = hostStr(opts.address)
		errCh     = make(chan error, 100)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			LocalHost:  localHost,
			RemotePort: "22",
		}, errCh); err != nil {
			errCh <- err
		}
		close(errCh)
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				slog.Debug("Tunnel error: %v", err)
			}
		}
	}()

	cli.PrintOut("SSH proxy is running on %s\n", localPort)
	cli.PrintOut(
		"\nConnect with: ssh -i %s/id_ed25519 ssh://root@%s:%s\n",
		cli.Config().SSHDirPath, localHost, localPort,
	)
	cli.PrintOut("\nOr add the following to your ~/.ssh/config:\n")
	cli.PrintOut("Host %s\n", opts.playID+"-"+opts.machine)
	cli.PrintOut("  HostName %s\n", localHost)
	cli.PrintOut("  Port %s\n", localPort)
	cli.PrintOut("  User root\n")
	cli.PrintOut("  IdentityFile %s/id_ed25519\n", cli.Config().SSHDirPath)
	cli.PrintOut("  StrictHostKeyChecking no\n")
	cli.PrintOut("  UserKnownHostsFile /dev/null\n")

	cli.PrintOut("\nPress Ctrl+C to stop\n")

	// Wait for ctrl+c
	<-ctx.Done()

	return nil
}

func portStr(address string) string {
	if address == "" {
		return portforward.RandomLocalPort()
	}

	if address[0] == ':' {
		return address[1:]
	}

	return strings.Split(address, ":")[1]
}

func hostStr(address string) string {
	if address == "" {
		return "127.0.0.1"
	}

	if address[0] == ':' {
		return "127.0.0.1"
	}

	return strings.Split(address, ":")[0]
}
