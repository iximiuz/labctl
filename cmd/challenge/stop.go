package challenge

import (
	"context"
	"fmt"
	"strings"

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
		Use:   "stop [flags] <challenge-url|challenge-name>",
		Short: `Stop the current solution attempt for a challenge`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.challenge = args[0]
			if strings.HasPrefix(opts.challenge, "https://") {
				parts := strings.Split(strings.Trim(opts.challenge, "/"), "/")
				opts.challenge = parts[len(parts)-1]
			}

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
	cli.PrintAux("Stopping the attempt for challenge %s...\n", opts.challenge)

	chal, err := cli.Client().GetChallenge(ctx, opts.challenge)
	if err != nil {
		return fmt.Errorf("couldn't get the challenge: %w", err)
	}

	if chal.Play == nil || !chal.Play.Active {
		cli.PrintErr("Challenge is not being attempted - nothing to stop.\n")
		return nil
	}

	if _, err = cli.Client().StopChallenge(ctx, opts.challenge); err != nil {
		return fmt.Errorf("couldn't stop the challenge: %w", err)
	}

	cli.PrintAux("Challenge attempt has been stopped.\n")
	return nil
}
