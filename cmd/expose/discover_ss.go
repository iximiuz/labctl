package expose

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

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
