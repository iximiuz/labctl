package playground

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

const waitPlaygroundTimeout = 48 * time.Hour

type waitOptions struct {
	initOnly bool
	timeout  time.Duration
}

func newWaitCommand(cli labcli.CLI) *cobra.Command {
	var opts waitOptions

	cmd := &cobra.Command{
		Use:               "wait [flags] <play-id>",
		Short:             "Wait until a playground's tasks are completed",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.NonDestroyedPlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runWaitPlayground(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVar(
		&opts.initOnly,
		"init-only",
		false,
		"Return as soon as all init tasks are completed (instead of waiting for all tasks)",
	)

	flags.DurationVar(
		&opts.timeout,
		"timeout",
		waitPlaygroundTimeout,
		"Maximum time to wait for the tasks to complete (0 to wait indefinitely)",
	)

	return cmd
}

func runWaitPlayground(ctx context.Context, cli labcli.CLI, playID string, opts *waitOptions) error {
	play, err := cli.Client().GetPlay(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if !play.IsActive() {
		return labcli.NewStatusError(1, "playground %s is not running", playID)
	}

	playConn := api.NewPlayConn(ctx, play, cli.Client(), cli.Config().WebSocketOrigin())
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	if err := playConn.WaitTasks(opts.timeout, opts.initOnly, spin); err != nil {
		if errors.Is(err, api.ErrPlayTasksFailed) {
			cli.PrintAux("Done waiting for playground tasks: one or more playground tasks failed.\n")
			return nil
		}
		return err
	}

	if opts.initOnly {
		cli.PrintAux("Done waiting for playground tasks: all init tasks completed.\n")
	} else {
		cli.PrintAux("Done waiting for playground tasks: all tasks completed.\n")
	}

	return nil
}
