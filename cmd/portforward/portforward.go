package portforward

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

type options struct {
	playID  string
	machine string

	locals       []string
	localsParsed []portforward.ForwardingSpec

	remotes []string

	quiet bool
}

// Local port forwarding's possible modes (kinda sorta as in ssh -L):
//   - REMOTE_PORT                                    # binds TARGET_IP:REMOTE_PORT to a random port on localhost
//   - REMOTE_HOST:REMOTE_PORT                        # binds arbitrary REMOTE_HOST:REMOTE_PORT to a random port on localhost
//   - LOCAL_PORT:REMOTE_PORT                         # much like the above form but uses a concrete port on the host system
//   - LOCAL_PORT:REMOTE_HOST:REMOTE_PORT             # the remote host is explicitly specified in addition to the port
//   - LOCAL_HOST:LOCAL_PORT:REMOTE_PORT              # similar to LOCAL_PORT:REMOTE_PORT but LOCAL_HOST is used instead of 127.0.0.1
//   - LOCAL_HOST:LOCAL_PORT:REMOTE_HOST:REMOTE_PORT  # the most explicit form

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:   "port-forward <playground> [-m machine] -L [LOCAL:]REMOTE [-L ...] | -R [REMOTE:]:LOCAL [-R ...]",
		Short: `Forward one or more local or remote ports to a running playground`,
		Long: `While the implementation for sure differs, the behavior and semantic of the command
are meant to be similar to SSH local (-L) and remote (-R) port forwarding. The word "local" always
refers to the labctl side. The word "remote" always refers to the target playground side.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(opts.locals)+len(opts.remotes) == 0 {
				return labcli.NewStatusError(1, "at least one -L or -R flag must be provided")
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

			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

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

	return cmd
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

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:   opts.playID,
		Machine:  opts.machine,
		PlaysDir: cli.Config().PlaysDir,
		SSHDir:   cli.Config().SSHDir,
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
