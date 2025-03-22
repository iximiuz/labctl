package expose

import (
	"context"
	"fmt"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type shellOptions struct {
	playID  string
	machine string
	user    string
	public  bool
	open    bool
}

func (o *shellOptions) access() api.AccessMode {
	if o.public {
		return api.AccessPublic
	}
	return api.AccessPrivate
}

func NewShellCommand(cli labcli.CLI) *cobra.Command {
	var opts shellOptions

	cmd := &cobra.Command{
		Use:   "shell <playground>",
		Short: "Expose a web terminal session with a handy URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]
			return labcli.WrapStatusError(runShell(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		"Target machine (default: the first machine in the playground)",
	)
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		"Username for the shell session (default: machine's default user)",
	)
	flags.BoolVarP(
		&opts.public,
		"public",
		"p",
		false,
		"Make the exposed shell URL publicly accessible",
	)
	flags.BoolVarP(
		&opts.open,
		"open",
		"o",
		false,
		"Open the exposed shell URL in browser",
	)

	return cmd
}

func runShell(ctx context.Context, cli labcli.CLI, opts *shellOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine == "" {
		opts.machine = p.Machines[0].Name
	} else if p.GetMachine(opts.machine) == nil {
		return fmt.Errorf("machine %q not found in the playground", opts.machine)
	}

	if opts.user == "" {
		opts.user = p.GetMachine(opts.machine).DefaultUser().Name
	}

	resp, err := cli.Client().ExposeShell(ctx, opts.playID, api.ExposeShellRequest{
		Machine: opts.machine,
		User:    opts.user,
		Access:  opts.access(),
	})
	if err != nil {
		return fmt.Errorf("couldn't expose shell: %w", err)
	}

	cli.PrintAux("Shell session %s@%s exposed as %s\n", resp.User, resp.Machine, resp.URL)

	if opts.open {
		cli.PrintAux("Opening %s in your browser...\n", resp.URL)

		if err := open.Run(resp.URL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the URL into a browser manually to access the shell.\n")
		}
	}

	cli.PrintOut("%s\n", resp.URL)
	return nil
}
