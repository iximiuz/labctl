package challenge

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type stopOptions struct {
	challenge string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [flags] <challenge-name>",
		Short: `Stop the current solution attempt for a challenge`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.challenge = args[0]

			return labcli.WrapStatusError(runStopChallenge(cmd.Context(), cli, &opts))
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

func runStopChallenge(ctx context.Context, cli labcli.CLI, opts *stopOptions) error {
	cli.PrintAux("Stopping solution attempt for challenge %s...\n", opts.challenge)

	chal, err := cli.Client().StopChallenge(ctx, opts.challenge)
	if err != nil {
		return fmt.Errorf("couldn't stop the challenge: %w", err)
	}

	cli.PrintAux("Challenge %s has been stopped.\n", chal.Name)

	return nil
}
