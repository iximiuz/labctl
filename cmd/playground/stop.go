package playground

import (
	"context"
	"fmt"
	"time"

	// "github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const stopCommandTimeout = 5 * time.Minute

type stopOptions struct {
	playIDS []string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [flags] <playground-ids>",
		Short: `Stop one or more playgrounds`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playIDS = args

			return labcli.WrapStatusError(runStopPlayground(cmd.Context(), cli, &opts))
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

func runStopPlayground(ctx context.Context, cli labcli.CLI, opts *stopOptions) error {
	for _, playID := range opts.playIDS {
		cli.PrintAux("Stopping playground %s...\n", playID)

		if err := cli.Client().DeletePlay(ctx, playID); err != nil {
			return fmt.Errorf("couldn't delete the playground: %w", err)
		}

		// s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
		// s.Writer = cli.AuxStream()
		// s.Prefix = "Waiting for playground " + playID + " to terminate... "
		// s.Start()

		ctx, cancel := context.WithTimeout(ctx, stopCommandTimeout)
		defer cancel()

		// for ctx.Err() == nil {

		if play, err := cli.Client().GetPlay(ctx, playID); err == nil && !play.Active {
			// s.FinalMSG = "Waiting for playground " + playID + " to terminate... Done.\n"
			// s.Stop()
			// time.Sleep(4 * time.Second)
			cli.PrintAux("Playground has been stopped.\n")

			return nil
		}

		// }

		// cli.PrintAux("Playground has been stopped.\n")
	}

	return nil
}
