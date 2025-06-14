package tutorial

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type stopOptions struct {
	tutorial string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [flags] <tutorial-url|tutorial-name>",
		Short: `Stop the current tutorial session`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.tutorial = args[0]
			if strings.HasPrefix(opts.tutorial, "https://") {
				parts := strings.Split(strings.Trim(opts.tutorial, "/"), "/")
				opts.tutorial = parts[len(parts)-1]
			}

			return labcli.WrapStatusError(runStopTutorial(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Do not print any diagnostic messages`,
	)

	return cmd
}

func runStopTutorial(ctx context.Context, cli labcli.CLI, opts *stopOptions) error {
	cli.PrintAux("Stopping the tutorial session for %s...\n", opts.tutorial)

	tut, err := cli.Client().GetTutorial(ctx, opts.tutorial)
	if err != nil {
		return fmt.Errorf("couldn't get the tutorial: %w", err)
	}

	if tut.Play == nil || !tut.Play.Active {
		cli.PrintErr("Tutorial is not running - nothing to stop.\n")
		return nil
	}

	if _, err = cli.Client().StopTutorial(ctx, opts.tutorial); err != nil {
		return fmt.Errorf("couldn't stop the tutorial: %w", err)
	}

	cli.PrintAux("Tutorial session has been stopped.\n")
	return nil
}
