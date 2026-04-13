package expose

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

const (
	tmpPlaygroundName   = "alpine"
	tmpPlaygroundWaitTo = 10 * time.Minute
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
		Use:   "local [<playground>] <local_addr>:<local_port> [--remote-port <port>]",
		Short: "Expose a local HTTP(s) endpoint as a public URL via a running playground",
		Long: `Expose a local HTTP(s) endpoint (running on the labctl side) as a URL, by combining
a remote port forward (labctl port-forward -R) with an HTTP(s) port exposure (labctl expose port).

When <playground> is omitted, a temporary Alpine playground is started under the
hood and destroyed when the command exits.

If --remote-port is not specified, the remote port defaults to <local_port>.`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			var addrArg string
			if len(args) == 2 {
				opts.playID = args[0]
				addrArg = args[1]
			} else {
				addrArg = args[0]
			}

			host, port, err := parseLocalAddr(addrArg)
			if err != nil {
				return labcli.NewStatusError(1, "invalid local address %q: %s", addrArg, err)
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
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	if opts.playID == "" {
		play, cleanup, err := startTmpPlayground(ctx, cli)
		if err != nil {
			return fmt.Errorf("couldn't start temporary playground: %w", err)
		}
		defer cleanup()
		opts.playID = play.ID
	}

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
	select {
	case err := <-doneCh:
		if err != nil {
			cli.PrintErr("Tunnel error: %v\n", err)
			exitErr = errors.Join(exitErr, err)
		}
	case <-ctx.Done():
		cli.PrintAux("\nShutting down...\n")
		if err := <-doneCh; err != nil && !errors.Is(err, context.Canceled) {
			cli.PrintErr("Tunnel error: %v\n", err)
			exitErr = errors.Join(exitErr, err)
		}
	}
	return exitErr
}

// startTmpPlayground creates an on-the-fly Alpine playground, waits for it to be
// ready, and returns a cleanup function that destroys it.
func startTmpPlayground(ctx context.Context, cli labcli.CLI) (*api.Play, func(), error) {
	cli.PrintAux("Starting a temporary %s playground...\n", tmpPlaygroundName)

	play, err := cli.Client().CreatePlay(ctx, api.CreatePlayRequest{
		Playground: tmpPlaygroundName,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't create playground: %w", err)
	}

	cleanup := func() {
		cli.PrintAux("Destroying temporary playground %s...\n", play.ID)
		// Use a fresh context so cleanup runs even after ctx cancellation.
		destroyCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := cli.Client().DestroyPlay(destroyCtx, play.ID); err != nil {
			cli.PrintErr("Warning: couldn't destroy temporary playground %s: %v\n", play.ID, err)
		}
	}

	cli.PrintAux("Temporary playground %s is ready\n", play.ID)
	return play, cleanup, nil
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
