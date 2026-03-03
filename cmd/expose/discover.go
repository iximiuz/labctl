package expose

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/retry"
	"github.com/iximiuz/labctl/internal/ssh"
)

type discoveredPort struct {
	Port      int
	Service   string
	Namespace string
}

func (d discoveredPort) Label() string {
	if d.Service != "" {
		return fmt.Sprintf("%s/%s (NodePort %d)", d.Namespace, d.Service, d.Port)
	}
	return strconv.Itoa(d.Port)
}

// connectSSH establishes a non-interactive SSH session to a playground machine
// by starting a WebSocket tunnel, forwarding port 22, and connecting via SSH.
// The caller must call the returned cancel function when done.
func connectSSH(
	ctx context.Context,
	cli labcli.CLI,
	play *api.Play,
	machine string,
) (*ssh.Session, context.CancelFunc, error) {
	user := "root"
	if m := play.GetMachine(machine); m != nil {
		if u := m.DefaultUser(); u != nil {
			user = u.Name
		}
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:          play.ID,
		Machine:         machine,
		SSHUser:         user,
		SSHIdentityFile: cli.Config().SSHIdentityFile,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't start tunnel to %s: %w", machine, err)
	}

	localPort := portforward.RandomLocalPort()
	errCh := make(chan error, 100)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			RemotePort: "22",
		}, errCh); err != nil {
			errCh <- err
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-errCh:
				if err != nil {
					slog.Debug("Tunnel error", "machine", machine, "error", err.Error())
				}
			}
		}
	}()

	var (
		dial net.Dialer
		conn net.Conn
		addr = "localhost:" + localPort
	)
	if err := retry.UntilSuccess(ctx, func() error {
		conn, err = dial.DialContext(ctx, "tcp", addr)
		return err
	}, 60, 1*time.Second); err != nil {
		cancel()
		return nil, nil, fmt.Errorf("couldn't connect to SSH on %s (%s): %w", machine, addr, err)
	}

	sess, err := ssh.NewSession(conn, user, cli.Config().SSHIdentityFile, false)
	if err != nil {
		cancel()
		conn.Close()
		return nil, nil, fmt.Errorf("couldn't create SSH session to %s: %w", machine, err)
	}

	return sess, cancel, nil
}

var skipPorts = map[int]bool{
	22: true, // SSH
	53: true, // DNS
}

var skipProcesses = map[string]bool{
	"systemd":          true,
	"systemd-resolve":  true,
	"systemd-resolved": true,
	"examiner":         true,
}

type ssEntry struct {
	port      int
	host      string
	processes string
}

// isLoopback returns true if the address is bound to localhost only.
func (e ssEntry) isLoopback() bool {
	return strings.HasPrefix(e.host, "127.") ||
		e.host == "[::1]"
}

// hasOnlySkippedProcesses returns true if every process name in the
// ss users:(...) column is in the skip list.
// Format: users:(("name",pid=N,fd=N),("name2",pid=N,fd=N))
func (e ssEntry) hasOnlySkippedProcesses() bool {
	if e.processes == "" {
		return false
	}
	names := extractProcessNames(e.processes)
	if len(names) == 0 {
		return false
	}
	for _, name := range names {
		if !skipProcesses[name] {
			return false
		}
	}
	return true
}

// extractProcessNames pulls process names from ss users:(...) format.
// e.g. users:(("sshd",pid=936,fd=3),("systemd",pid=1,fd=63)) -> ["sshd", "systemd"]
func extractProcessNames(usersField string) []string {
	var names []string
	s := usersField
	for {
		start := strings.Index(s, "(\"")
		if start < 0 {
			break
		}
		s = s[start+2:]
		end := strings.Index(s, "\"")
		if end < 0 {
			break
		}
		names = append(names, s[:end])
		s = s[end+1:]
	}
	return names
}

// parseSSOutput parses the output of `ss -lntp` and returns a deduplicated,
// sorted list of listening TCP port numbers, filtering out:
//   - well-known system ports (SSH, DNS)
//   - localhost-only bindings (127.x.x.x, [::1])
//   - known system processes (systemd, systemd-resolve, examiner)
func parseSSOutput(output string) ([]int, error) {
	entries := make(map[int]ssEntry)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || !strings.HasPrefix(line, "LISTEN") {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		addr := fields[3]
		host, port, err := extractHostPort(addr)
		if err != nil {
			slog.Debug("Skipping unparseable address in ss output", "addr", addr, "error", err)
			continue
		}

		var processes string
		if len(fields) >= 6 {
			processes = fields[5]
		}

		entry := ssEntry{port: port, host: host, processes: processes}

		if skipPorts[port] {
			slog.Debug("Skipping system port", "port", port)
			continue
		}

		if existing, ok := entries[port]; ok {
			if !existing.isLoopback() {
				continue
			}
			if entry.isLoopback() {
				continue
			}
		}
		entries[port] = entry
	}

	var ports []int
	for _, e := range entries {
		if e.isLoopback() {
			slog.Debug("Skipping loopback-only port", "port", e.port, "host", e.host)
			continue
		}
		if e.hasOnlySkippedProcesses() {
			slog.Debug("Skipping system process port", "port", e.port, "processes", e.processes)
			continue
		}
		ports = append(ports, e.port)
	}

	sort.Ints(ports)
	return ports, nil
}

// extractHostPort splits an ss local address into the host part and port number.
// Handles IPv4 (0.0.0.0:PORT), IPv6 ([::]:PORT), and wildcard (*:PORT).
func extractHostPort(addr string) (string, int, error) {
	if strings.HasPrefix(addr, "[") {
		idx := strings.LastIndex(addr, "]:")
		if idx < 0 {
			return "", 0, fmt.Errorf("no port separator in IPv6 address %q", addr)
		}
		host := addr[:idx+1]
		port, err := strconv.Atoi(addr[idx+2:])
		return host, port, err
	}
	idx := strings.LastIndex(addr, ":")
	if idx < 0 {
		return "", 0, fmt.Errorf("no port separator in address %q", addr)
	}
	host := addr[:idx]
	port, err := strconv.Atoi(addr[idx+1:])
	return host, port, err
}

// kubeServiceList represents the minimal structure of `kubectl get svc -A -o json`.
type kubeServiceList struct {
	Items []kubeService `json:"items"`
}

type kubeService struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Type  string     `json:"type"`
		Ports []kubePort `json:"ports"`
	} `json:"spec"`
}

type kubePort struct {
	NodePort int `json:"nodePort"`
}

func parseKubeSvcOutput(output []byte) ([]discoveredPort, error) {
	var svcList kubeServiceList
	if err := json.Unmarshal(output, &svcList); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	var ports []discoveredPort
	seen := make(map[int]bool)
	for _, svc := range svcList.Items {
		if svc.Spec.Type != "NodePort" {
			continue
		}
		for _, p := range svc.Spec.Ports {
			if p.NodePort == 0 || seen[p.NodePort] {
				continue
			}
			seen[p.NodePort] = true
			ports = append(ports, discoveredPort{
				Port:      p.NodePort,
				Service:   svc.Metadata.Name,
				Namespace: svc.Metadata.Namespace,
			})
		}
	}

	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Port < ports[j].Port
	})
	return ports, nil
}

// kubeNodeList represents the minimal structure of `kubectl get nodes -o json`.
type kubeNodeList struct {
	Items []kubeNode `json:"items"`
}

type kubeNode struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
}

// pickExposeNode selects a playground machine to expose NodePorts on by
// cross-referencing kubectl node names with playground machine names.
// Falls back to the first playground machine if no match is found.
func pickExposeNode(nodesJSON []byte, playMachines []api.Machine) (string, error) {
	var nodeList kubeNodeList
	if err := json.Unmarshal(nodesJSON, &nodeList); err != nil {
		return "", fmt.Errorf("failed to parse kubectl nodes output: %w", err)
	}

	machineNames := make(map[string]bool, len(playMachines))
	for _, m := range playMachines {
		machineNames[m.Name] = true
	}

	for _, node := range nodeList.Items {
		if machineNames[node.Metadata.Name] {
			return node.Metadata.Name, nil
		}
	}

	if len(playMachines) > 0 {
		return playMachines[0].Name, nil
	}

	return "", fmt.Errorf("no machines available in playground")
}

func runAutoExpose(ctx context.Context, cli labcli.CLI, opts *portOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	var machines []string
	if opts.machine != "" {
		if p.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the playground", opts.machine)
		}
		machines = []string{opts.machine}
	} else if opts.allMachines {
		for _, m := range p.Machines {
			machines = append(machines, m.Name)
		}
	} else {
		machines = []string{p.Machines[0].Name}
	}

	alreadyExposed, err := cli.Client().ListPorts(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't list existing exposed ports: %w", err)
	}
	exposedSet := make(map[string]bool)
	for _, port := range alreadyExposed {
		exposedSet[fmt.Sprintf("%s:%d", port.Machine, port.Number)] = true
	}

	for _, machine := range machines {
		cli.PrintAux("Discovering listening ports on %s...\n", machine)

		sess, cancel, err := connectSSH(ctx, cli, p, machine)
		if err != nil {
			cli.PrintErr("Warning: couldn't connect to %s: %v\n", machine, err)
			continue
		}

		output, err := sess.RunOutput(ctx, "sudo ss -lntp")
		sess.Close()
		cancel()
		if err != nil {
			cli.PrintErr("Warning: couldn't run ss on %s: %v\n", machine, err)
			continue
		}

		ports, err := parseSSOutput(string(output))
		if err != nil {
			cli.PrintErr("Warning: couldn't parse ss output from %s: %v\n", machine, err)
			continue
		}

		if len(ports) == 0 {
			cli.PrintAux("No listening ports found on %s\n", machine)
			continue
		}

		cli.PrintAux("Found %d listening port(s) on %s: %v\n", len(ports), machine, ports)

		for _, port := range ports {
			key := fmt.Sprintf("%s:%d", machine, port)
			if exposedSet[key] {
				cli.PrintAux("Skipping %s:%d (already exposed)\n", machine, port)
				continue
			}

			resp, err := cli.Client().ExposePort(ctx, opts.playID, api.ExposePortRequest{
				Machine: machine,
				Number:  port,
				Access:  opts.access(),
			})
			if err != nil {
				cli.PrintErr("Warning: couldn't expose port %d on %s: %v\n", port, machine, err)
				continue
			}

			cli.PrintAux("HTTP port %s:%d exposed as %s\n", resp.Machine, resp.Number, resp.URL)
			cli.PrintOut("%s\n", resp.URL)
		}
	}

	return nil
}

func runK8sExpose(ctx context.Context, cli labcli.CLI, opts *portOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	kubectlMachine := p.Machines[0].Name
	cli.PrintAux("Connecting to %s to run kubectl...\n", kubectlMachine)

	sess, cancel, err := connectSSH(ctx, cli, p, kubectlMachine)
	if err != nil {
		return fmt.Errorf("couldn't connect to %s: %w", kubectlMachine, err)
	}
	defer cancel()

	kubectlSvcCmd := "kubectl get svc --all-namespaces -o json"
	if opts.namespace != "" {
		kubectlSvcCmd = fmt.Sprintf("kubectl get svc -n %s -o json", opts.namespace)
		cli.PrintAux("Discovering NodePort services in namespace %q...\n", opts.namespace)
	} else {
		cli.PrintAux("Discovering NodePort services across all namespaces...\n")
	}

	svcOutput, err := sess.RunOutput(ctx, kubectlSvcCmd)
	if err != nil {
		sess.Close()
		return fmt.Errorf("couldn't run kubectl get svc on %s: %w", kubectlMachine, err)
	}

	discoveredPorts, err := parseKubeSvcOutput(svcOutput)
	if err != nil {
		sess.Close()
		return err
	}

	if len(discoveredPorts) == 0 {
		sess.Close()
		cli.PrintAux("No NodePort services found in the cluster\n")
		return nil
	}

	cli.PrintAux("Found %d NodePort service port(s)\n", len(discoveredPorts))

	exposeMachine := opts.machine
	if exposeMachine == "" {
		nodesOutput, err := sess.RunOutput(ctx, "kubectl get nodes -o json")
		sess.Close()
		if err != nil {
			return fmt.Errorf("couldn't run kubectl get nodes on %s: %w", kubectlMachine, err)
		}

		exposeMachine, err = pickExposeNode(nodesOutput, p.Machines)
		if err != nil {
			return fmt.Errorf("couldn't determine which node to expose on: %w", err)
		}
		cli.PrintAux("Selected node %s for exposing NodePorts\n", exposeMachine)
	} else {
		sess.Close()
		if p.GetMachine(exposeMachine) == nil {
			return fmt.Errorf("machine %q not found in the playground", exposeMachine)
		}
	}

	alreadyExposed, err := cli.Client().ListPorts(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't list existing exposed ports: %w", err)
	}
	exposedSet := make(map[string]bool)
	for _, port := range alreadyExposed {
		exposedSet[fmt.Sprintf("%s:%d", port.Machine, port.Number)] = true
	}

	for _, dp := range discoveredPorts {
		key := fmt.Sprintf("%s:%d", exposeMachine, dp.Port)
		if exposedSet[key] {
			cli.PrintAux("Skipping %s (already exposed)\n", dp.Label())
			continue
		}

		cli.PrintAux("Exposing %s...\n", dp.Label())

		resp, err := cli.Client().ExposePort(ctx, opts.playID, api.ExposePortRequest{
			Machine: exposeMachine,
			Number:  dp.Port,
			Access:  opts.access(),
		})
		if err != nil {
			cli.PrintErr("Warning: couldn't expose %s: %v\n", dp.Label(), err)
			continue
		}

		cli.PrintAux("HTTP port %s:%d exposed as %s\n", resp.Machine, resp.Number, resp.URL)
		cli.PrintOut("%s\n", resp.URL)
	}

	return nil
}
