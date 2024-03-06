package playground

import (
	"context"
	"fmt"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type startOptions struct {
	playground string

	open  bool
	quiet bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <playground-name>",
		Short: `Start a new playground, possibly open it in a browser`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playground = args[0]

			return labcli.WrapStatusError(runStartPlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(
		&opts.open,
		"open",
		false,
		`Open the playground page in a browser`,
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

	cli.PrintAux("Opening %s in your browser...\n", play.PageURL)

	if err := open.Run(play.PageURL); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
	}

	cli.PrintOut("%s\n", play.ID)

	return nil
}
