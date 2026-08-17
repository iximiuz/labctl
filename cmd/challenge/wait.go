package challenge

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

const (
	waitChallengeTimeout      = 5 * time.Minute
	waitChallengePollInterval = 2 * time.Second
)

type waitChallengeOptions struct {
	timeout time.Duration
}

func newWaitCommand(cli labcli.CLI) *cobra.Command {
	var opts waitChallengeOptions

	cmd := &cobra.Command{
		Use:               "wait [flags] <challenge-name>",
		Short:             "Wait until the challenge is marked as solved (the completion is recorded server-side)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.StartedChallengeNames(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runWaitChallenge(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()

	flags.DurationVar(
		&opts.timeout,
		"timeout",
		waitChallengeTimeout,
		"Maximum time to wait for the challenge to be marked as solved (0 to wait indefinitely)",
	)

	return cmd
}

func runWaitChallenge(ctx context.Context, cli labcli.CLI, name string, opts *waitChallengeOptions) error {
	cli.PrintAux("Waiting for challenge %s to be marked as solved...\n", name)

	if err := waitChallengeSolved(ctx, cli, name, opts.timeout); err != nil {
		return err
	}

	cli.PrintAux("Challenge %s has been completed.\n", name)

	return nil
}

// waitChallengeSolved polls the API until the server-recorded completion of
// the challenge becomes visible. Foreman records completions on its own once
// all tasks pass (realtime for SSE-status play servers, within a few seconds
// for polled ones), so on a healthy platform this returns almost immediately
// after the tasks are done.
func waitChallengeSolved(ctx context.Context, cli labcli.CLI, name string, timeout time.Duration) error {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	var lastErr error
	for {
		chal, err := cli.Client().GetChallenge(ctx, name)
		if err == nil && chal.CompletedAt != "" {
			return nil
		}
		lastErr = err

		select {
		case <-ctx.Done():
			if lastErr != nil {
				return fmt.Errorf("couldn't confirm the challenge completion: %w", lastErr)
			}
			return labcli.NewStatusError(1, "challenge %s hasn't been marked as solved in time", name)
		case <-time.After(waitChallengePollInterval):
		}
	}
}
