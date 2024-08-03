package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const stopCommandTimeout = 5 * time.Minute

type stopOptions struct {
	playID string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [flags] <playground-id>",
		Short: `Stop one or more playgrounds`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

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
	cli.PrintAux("Stopping playground %s...\n", opts.playID)

	if err := cli.Client().DeletePlay(ctx, opts.playID); err != nil {
		return fmt.Errorf("couldn't delete the playground: %w", err)
	}

	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for playground to terminate... "
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, stopCommandTimeout)
	defer cancel()

	for ctx.Err() == nil {
		if play, err := cli.Client().GetPlay(ctx, opts.playID); err == nil && !play.Active {
			s.FinalMSG = "Waiting for playground to terminate... Done.\n"
			s.Stop()

			return nil
		}

		time.Sleep(2 * time.Second)
	}

	cli.PrintAux("Playground has been stopped.\n")

	return nil
}
