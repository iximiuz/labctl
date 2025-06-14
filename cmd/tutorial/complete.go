package tutorial

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type completeOptions struct {
	tutorial string

	quiet bool
}

func newCompleteCommand(cli labcli.CLI) *cobra.Command {
	var opts completeOptions

	cmd := &cobra.Command{
		Use:    "complete [flags] <tutorial-name>",
		Short:  `Mark a tutorial as completed`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.tutorial = args[0]

			return labcli.WrapStatusError(runCompleteTutorial(cmd.Context(), cli, &opts))
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

func runCompleteTutorial(ctx context.Context, cli labcli.CLI, opts *completeOptions) error {
	cli.PrintAux("Marking tutorial %s as completed...\n", opts.tutorial)

	tut, err := cli.Client().CompleteTutorial(ctx, opts.tutorial)
	if err != nil {
		return fmt.Errorf("couldn't complete the tutorial: %w", err)
	}

	cli.PrintAux("Tutorial %s has been completed.\n", tut.Name)
	return nil
}
