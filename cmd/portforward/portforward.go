package portforward

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

// runRestorePortForwards restores saved port forwards and blocks until done.
func runRestorePortForwards(ctx context.Context, cli labcli.CLI, opts *options) error {
	resultCh, err := portforward.RestoreSavedForwards(ctx, cli.Client(), opts.playID, cli)
	if err != nil {
		return err
	}

	return <-resultCh
}

type options struct {
	playID  string
	machine string

	locals       []string
	localsParsed []portforward.ForwardingSpec

	remotes []string

	quiet bool

	// New flags
	list    bool
	restore bool
	remove  int
}

// Local port forwarding's possible modes (kinda sorta as in ssh -L):
//   - REMOTE_PORT                                    # binds TARGET_IP:REMOTE_PORT to a random port on localhost
//   - REMOTE_HOST:REMOTE_PORT                        # binds arbitrary REMOTE_HOST:REMOTE_PORT to a random port on localhost
//   - LOCAL_PORT:REMOTE_PORT                         # much like the above form but uses a concrete port on the host system
//   - LOCAL_PORT:REMOTE_HOST:REMOTE_PORT             # the remote host is explicitly specified in addition to the port
//   - LOCAL_HOST:LOCAL_PORT:REMOTE_PORT              # similar to LOCAL_PORT:REMOTE_PORT but LOCAL_HOST is used instead of 127.0.0.1
//   - LOCAL_HOST:LOCAL_PORT:REMOTE_HOST:REMOTE_PORT  # the most explicit form

func NewCommand(cli labcli.CLI) *cobra.Command {
	opts := options{
		remove: -1, // -1 means not set
	}

	cmd := &cobra.Command{
		Use:   "port-forward <playground> [-m machine] -L [LOCAL:]REMOTE [-L ...] | --list | --restore | --remove <index>",
		Short: `Forward one or more local or remote ports to a running playground`,
		Long: `Forward one or more local or remote ports to a running playground.

While the implementation differs significantly, the behavior and semantic of the command
are meant to be similar to SSH local (-L) and remote (-R) port forwarding. The word "local" always
refers to the labctl side. The word "remote" always refers to the target playground side.

The command also supports managing "saved" port forwards:
  --list     List all "should be forwarded" ports for the playground
  --restore  Forward all "should be forwarded" ports (handy after a persistent playground restart)
  --remove   Remove a "should be forwarded" port from the playground's config by its index (0-based)

When using -L|-R flags, port forwards are automatically saved to the playground's config for later restoration.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]
			cli.SetQuiet(opts.quiet)

			// Handle list mode
			if opts.list {
				return labcli.WrapStatusError(runListPortForwards(cmd.Context(), cli, &opts))
			}

			// Handle remove mode
			if opts.remove >= 0 {
				return labcli.WrapStatusError(runRemovePortForward(cmd.Context(), cli, &opts))
			}

			// Handle restore mode
			if opts.restore {
				return labcli.WrapStatusError(runRestorePortForwards(cmd.Context(), cli, &opts))
			}

			// Regular port forwarding mode
			if len(opts.locals)+len(opts.remotes) == 0 {
				return labcli.NewStatusError(1, "at least one -L or -R flag must be provided (or use --list, --restore, --remove)")
			}
			if len(opts.remotes) > 0 {
				// TODO: Implement me!
				return labcli.NewStatusError(1, "remote port forwarding is not implemented yet")
			}

			for _, local := range opts.locals {
				parsed, err := portforward.ParseLocal(local)
				if err != nil {
					return labcli.NewStatusError(1, "invalid local port forwarding spec: %s", local)
				}
				opts.localsParsed = append(opts.localsParsed, parsed)
			}

			return labcli.WrapStatusError(runPortForward(cmd.Context(), cli, &opts))
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
	flags.StringSliceVarP(
		&opts.locals,
		"local",
		"L",
		nil,
		`Local port forwarding in the form [[LOCAL_HOST:]LOCAL_PORT:][REMOTE_HOST:]REMOTE_PORT`,
	)
	flags.StringSliceVarP(
		&opts.remotes,
		"remote",
		"R",
		nil,
		`Remote port forwarding in the form [REMOTE_HOST:]REMOTE_PORT:LOCAL_HOST:LOCAL_PORT`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Suppress verbose output`,
	)

	flags.BoolVar(&opts.list, "list", false, `List saved port forwards ("saved" means "should be forwarded")`)
	flags.BoolVar(&opts.restore, "restore", false, `Forward all "should be forwarded" ports for the playground`)
	flags.IntVar(&opts.remove, "remove", -1, `Remove a "should be forwarded" port from the playground's config by index (0-based)`)

	return cmd
}

func runListPortForwards(ctx context.Context, cli labcli.CLI, opts *options) error {
	forwards, err := cli.Client().ListPortForwards(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't list port forwards: %w", err)
	}

	if len(forwards) == 0 {
		cli.PrintAux("No saved port forwards found.\n")
		return nil
	}

	cli.PrintAux("Saved port forwards:\n")
	for i, pf := range forwards {
		localPart := ""
		if pf.LocalHost != "" || pf.LocalPort > 0 {
			if pf.LocalHost != "" {
				localPart = pf.LocalHost
			}
			if pf.LocalPort > 0 {
				if localPart != "" {
					localPart += ":"
				}
				localPart += strconv.Itoa(pf.LocalPort)
			}
			localPart += " -> "
		}

		remotePart := ""
		if pf.RemoteHost != "" {
			remotePart = pf.RemoteHost + ":"
		}
		if pf.RemotePort > 0 {
			remotePart += strconv.Itoa(pf.RemotePort)
		}

		kindLabel := pf.Kind
		if kindLabel == "" {
			kindLabel = "local"
		}

		cli.PrintAux("  [%d] %s (%s): %s%s\n", i, pf.Machine, kindLabel, localPart, remotePart)
	}

	return nil
}

func runRemovePortForward(ctx context.Context, cli labcli.CLI, opts *options) error {
	if err := cli.Client().RemovePortForward(ctx, opts.playID, opts.remove); err != nil {
		return fmt.Errorf("couldn't remove port forward: %w", err)
	}

	cli.PrintAux("Port forward at index %d removed.\n", opts.remove)
	return nil
}

func runPortForward(ctx context.Context, cli labcli.CLI, opts *options) error {
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

	// Save port forwards to play's config
	for _, spec := range opts.localsParsed {
		pf, err := spec.ToPortForward(opts.machine)
		if err != nil {
			return fmt.Errorf("couldn't convert port forwarding spec to API port forward model: %w", err)
		}
		if _, err := cli.Client().AddPortForward(ctx, p.ID, *pf); err != nil {
			cli.PrintErr("Warning: couldn't save port forward: %v\n", err)
		}
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:  p.ID,
		Machine: opts.machine,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		g     errgroup.Group
		errCh = make(chan error, 100)
	)
	for _, spec := range opts.localsParsed {
		spec := spec

		g.Go(func() error {
			cli.PrintAux("Forwarding %s -> %s\n", spec.LocalAddr(), spec.RemoteAddr())

			return tunnel.Forward(ctx, spec, errCh)
		})
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				cli.PrintErr("Tunnel error: %v", err)
			}
		}
	}()

	if err := g.Wait(); err != nil {
		return err
	}

	return nil
}
