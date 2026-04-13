package expose

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

type localOptions struct {
	playID string

	localHost string
	localPort string

	remotePort int

	machine string

	https  bool
	public bool
	open   bool
	quiet  bool
}

func (o *localOptions) access() api.AccessMode {
	if o.public {
		return api.AccessPublic
	}
	return api.AccessPrivate
}

func (o *localOptions) protocol() string {
	if o.https {
		return "HTTPS"
	}
	return "HTTP"
}

func NewLocalCommand(cli labcli.CLI) *cobra.Command {
	var opts localOptions

	cmd := &cobra.Command{
		Use:   "local <playground> <local_addr>:<local_port> [--remote-port <port>]",
		Short: "Expose a local HTTP(s) endpoint as a public URL via a running playground",
		Long: `Expose a local HTTP(s) endpoint (running on the labctl side) as a URL, by combining
a remote port forward (labctl port-forward -R) with an HTTP(s) port exposure (labctl expose port).

If --remote-port is not specified, the remote port defaults to <local_port>.`,
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

			host, port, err := parseLocalAddr(args[1])
			if err != nil {
				return labcli.NewStatusError(1, "invalid local address %q: %s", args[1], err)
			}
			opts.localHost = host
			opts.localPort = port

			if opts.remotePort == 0 {
				p, err := strconv.Atoi(opts.localPort)
				if err != nil {
					return labcli.NewStatusError(1, "couldn't derive remote port from local port %q: %s", opts.localPort, err)
				}
				opts.remotePort = p
			}
			if opts.remotePort < 1 || opts.remotePort > 65535 {
				return labcli.NewStatusError(1, "invalid remote port number (must be between 1 and 65535): %d", opts.remotePort)
			}

			return labcli.WrapStatusError(runLocal(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()
	flags.IntVar(
		&opts.remotePort,
		"remote-port",
		0,
		"Remote port to bind on the playground (default: same as the local port)",
	)
	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		"Target machine (default: the first machine in the playground)",
	)
	flags.BoolVarP(
		&opts.https,
		"https",
		"s",
		false,
		"Enable if the local service uses HTTPS (including self-signed certificates)",
	)
	flags.BoolVarP(
		&opts.public,
		"public",
		"p",
		false,
		"Make the exposed service publicly accessible",
	)
	flags.BoolVarP(
		&opts.open,
		"open",
		"o",
		false,
		"Open the exposed service in browser",
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		"Suppress verbose output",
	)

	return cmd
}

func runLocal(ctx context.Context, cli labcli.CLI, opts *localOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine, err = p.ResolveMachine(opts.machine); err != nil {
		return err
	}

	resp, err := cli.Client().ExposePort(ctx, opts.playID, api.ExposePortRequest{
		Machine: opts.machine,
		Number:  opts.remotePort,
		Access:  opts.access(),
		TLS:     opts.https,
	})
	if err != nil {
		return fmt.Errorf("couldn't expose port: %w", err)
	}

	cli.PrintAux("%s port %s:%d exposed as %s\n", opts.protocol(), resp.Machine, resp.Number, resp.URL)

	spec := portforward.ForwardingSpec{
		Kind:       "remote",
		RemoteHost: "0.0.0.0",
		RemotePort: strconv.Itoa(opts.remotePort),
		LocalHost:  opts.localHost,
		LocalPort:  opts.localPort,
	}

	pf, err := spec.ToPortForward(opts.machine)
	if err != nil {
		return fmt.Errorf("couldn't convert port forwarding spec to API port forward model: %w", err)
	}
	if err := cli.Client().AddPortForward(ctx, p.ID, *pf); err != nil {
		cli.PrintErr("Warning: couldn't save port forward: %v\n", err)
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:  p.ID,
		Machine: opts.machine,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	cli.PrintAux("Forwarding %s (remote) -> %s (local)\n", spec.RemoteAddr(), spec.LocalAddr())
	doneCh := tunnel.StartForwarding(ctx, spec)

	if opts.open {
		browser.OpenWithFallbackMessage(cli, resp.URL)
	}

	cli.PrintOut("%s\n", resp.URL)

	var exitErr error
	if err := <-doneCh; err != nil {
		cli.PrintErr("Tunnel error: %v", err)
		exitErr = errors.Join(exitErr, err)
	}
	return exitErr
}

// parseLocalAddr parses a <local_addr>:<local_port> string. The host part is
// optional and defaults to 127.0.0.1 when only a port is given. IPv6 addresses
// must be wrapped in square brackets (e.g., [::1]:3000).
func parseLocalAddr(s string) (string, string, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", "", fmt.Errorf("empty address")
	}

	var host, port string
	if strings.HasPrefix(s, "[") {
		end := strings.Index(s, "]")
		if end < 0 {
			return "", "", fmt.Errorf("missing closing bracket for IPv6 host")
		}
		host = s[1:end]
		rest := s[end+1:]
		if !strings.HasPrefix(rest, ":") {
			return "", "", fmt.Errorf("missing port after IPv6 host")
		}
		port = rest[1:]
	} else if idx := strings.LastIndex(s, ":"); idx >= 0 {
		host = s[:idx]
		port = s[idx+1:]
	} else {
		host = ""
		port = s
	}

	if host == "" {
		host = "127.0.0.1"
	}
	if port == "" {
		return "", "", fmt.Errorf("missing port")
	}
	if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
		return "", "", fmt.Errorf("invalid port %q", port)
	}

	return host, port, nil
}
