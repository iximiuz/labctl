package challenge

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type completeOptions struct {
	challenge string

	quiet bool
}

func newCompleteCommand(cli labcli.CLI) *cobra.Command {
	var opts completeOptions

	cmd := &cobra.Command{
		Use:    "complete [flags] <challenge-name>",
		Short:  `Try to complete a challenge (all tasks must be solved first)`,
		Hidden: true,
		Args:   cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.challenge = args[0]

			return labcli.WrapStatusError(runCompleteChallenge(cmd.Context(), cli, &opts))
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

func runCompleteChallenge(ctx context.Context, cli labcli.CLI, opts *completeOptions) error {
	cli.PrintAux("Trying to complete challenge %s...\n", opts.challenge)

	chal, err := cli.Client().CompleteChallenge(ctx, opts.challenge)
	if err != nil {
		return fmt.Errorf("couldn't complete the challenge: %w", err)
	}

	cli.PrintAux("Challenge %s has been completed.\n", chal.Name)

	return nil
}
