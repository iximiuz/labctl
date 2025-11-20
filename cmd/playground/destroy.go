package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const destroyCommandTimeout = 5 * time.Minute

type destroyOptions struct {
	playID string

	quiet bool
}

func newDestroyCommand(cli labcli.CLI) *cobra.Command {
	var opts destroyOptions

	cmd := &cobra.Command{
		Use:   "destroy [flags] <playground-id>",
		Short: `Destroy an active or stopped playground session, completely deleting its data`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

			return labcli.WrapStatusError(runDestroyPlayground(cmd.Context(), cli, &opts))
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

func runDestroyPlayground(ctx context.Context, cli labcli.CLI, opts *destroyOptions) error {
	cli.PrintAux("Destroying playground %s...\n", opts.playID)

	if err := cli.Client().DestroyPlay(ctx, opts.playID); err != nil {
		return fmt.Errorf("couldn't destroy the playground: %w", err)
	}

	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for playground to be destroyed... "
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, destroyCommandTimeout)
	defer cancel()

	for ctx.Err() == nil {
		if play, err := cli.Client().GetPlay(ctx, opts.playID); err == nil && !play.IsActive() {
			s.FinalMSG = "Waiting for playground to be destroyed... Done.\n"
			s.Stop()

			return nil
		}

		time.Sleep(2 * time.Second)
	}

	cli.PrintAux("Playground has been destroyed.\n")

	return nil
}
