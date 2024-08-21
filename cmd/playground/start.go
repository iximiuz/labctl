package playground

import (
	"context"
	"fmt"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type startOptions struct {
	playground string
	machine    string
	user       string

	open bool

	ssh bool

	ide bool

	quiet bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <playground-name>",
		Short: `Start a new playground, possibly open it in a browser`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"playground name is required\n\nAvailable playgrounds:\n%s",
					listKnownPlaygrounds(cmd.Context(), cli))
			}

			if opts.ide && opts.ssh {
				return labcli.NewStatusError(1, "can't use --ide and --ssh flags at the same time")
			}

			opts.playground = args[0]

			return labcli.WrapStatusError(runStartPlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.machine,
		"machine",
		"",
		`SSH into the machine with the given name (requires --ssh flag, default to the first machine)`,
	)
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		`SSH user (default: the machine's default login user)`,
	)

	flags.BoolVar(
		&opts.open,
		"open",
		false,
		`Open the playground page in a browser`,
	)
	flags.BoolVar(
		&opts.ssh,
		"ssh",
		false,
		`SSH into the playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.ide,
		"ide",
		false,
		`Open the playground in the IDE (only VSCode is supported at the moment)`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print playground's ID`,
	)

	return cmd
}

func runStartPlayground(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	play, err := cli.Client().CreatePlay(ctx, api.CreatePlayRequest{
		Playground: opts.playground,
	})
	if err != nil {
		return fmt.Errorf("couldn't create a new playground: %w", err)
	}

	cli.PrintAux("New %s playground created with ID %s\n", opts.playground, play.ID)

	if opts.open {
		cli.PrintAux("Opening %s in your browser...\n", play.PageURL)

		if err := open.Run(play.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	if opts.ide {
		return sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
			PlayID:  play.ID,
			Machine: opts.machine,
			User:    opts.user,
			IDE:     true,
		})
	}

	if opts.ssh {
		if opts.machine == "" {
			opts.machine = play.Machines[0].Name
		} else {
			if play.GetMachine(opts.machine) == nil {
				return fmt.Errorf("machine %q not found in the playground", opts.machine)
			}
		}

		cli.PrintAux("SSH-ing into %s machine...\n", opts.machine)

		return ssh.RunSSHSession(ctx, cli, play.ID, opts.machine, opts.user, nil)
	}

	cli.PrintOut("%s\n", play.ID)

	return nil
}

func listKnownPlaygrounds(ctx context.Context, cli labcli.CLI) string {
	playgrounds, err := cli.Client().ListPlaygrounds(ctx)
	if err != nil {
		cli.PrintErr("Couldn't list known playgrounds: %v\n", err)
		return ""
	}

	var res []string
	for _, p := range playgrounds {
		res = append(res, fmt.Sprintf("  - %s - %s", p.Name, p.Description))
	}

	return strings.Join(res, "\n")
}
