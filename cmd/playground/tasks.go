package playground

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/cenkalti/backoff/v5"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type tasksOptions struct {
	output   string
	wait     bool
	failFast bool
	timeout  time.Duration
}

func (opts *tasksOptions) validate() error {
	if opts.output != "table" && opts.output != "json" && opts.output != "name" && opts.output != "none" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json, name, none)", opts.output)
	}

	return nil
}

func newTasksCommand(cli labcli.CLI) *cobra.Command {
	var opts tasksOptions

	cmd := &cobra.Command{
		Use:   "tasks <play-id>",
		Short: "List tasks of a running playground",
		Args:  cobra.ExactArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListTasks(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.output,
		"output",
		"o",
		"table",
		"Output format: table, json, name, none",
	)

	flags.BoolVar(
		&opts.wait,
		"wait",
		false,
		"Wait for each task to reach its final state",
	)

	flags.BoolVar(
		&opts.failFast,
		"fail-fast",
		false,
		"If any of the tasks fail while waiting, exit with an error immediately",
	)

	flags.DurationVar(
		&opts.timeout,
		"timeout",
		30*time.Second,
		"Timeout to wait for tasks to finish",
	)

	return cmd
}

func runListTasks(ctx context.Context, cli labcli.CLI, playgroundID string, opts *tasksOptions) error {
	var (
		errFailed     = errors.New("some tasks failed")
		errUnfinished = errors.New("timed out waiting for tasks to finish")
	)
	operation := func() (*api.Play, error) {
		play, err := cli.Client().GetPlay(ctx, playgroundID)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("couldn't get playground: %w", err))
		}

		if !opts.wait {
			return play, nil
		}

		var failed, unfinished bool

		for _, task := range play.Tasks {
			if task.Status == api.PlayTaskStatusFailed {
				failed = true

				break
			}

			if !taskIsFinished(task) {
				unfinished = true
			}
		}

		if failed && opts.failFast {
			return play, backoff.Permanent(errFailed)
		} else if failed {
			return play, errFailed
		} else if unfinished {
			return play, errUnfinished
		}

		return play, nil
	}

	play, err := backoff.Retry(
		ctx,
		operation,
		backoff.WithMaxElapsedTime(opts.timeout),
		backoff.WithBackOff(backoff.NewExponentialBackOff()),
	)
	if err != nil && !errors.Is(err, errFailed) && !errors.Is(err, errUnfinished) {
		return err
	}

	if opts.output != "none" {
		printer := newPrinter(cli.OutputStream(), opts.output)

		if err := printer.Print(play.Tasks); err != nil {
			return err
		}
		defer printer.Flush()
	}

	if err != nil {
		return labcli.NewStatusError(2, err.Error())
	}

	return nil
}

type printer interface {
	Print(map[string]api.PlayTask) error
	Flush()
}

func newPrinter(w io.Writer, output string) printer {
	switch output {
	case "table":
		header := []string{
			"NAME",
			"STATUS",
			"INIT",
			"HELPER",
		}

		rowFunc := func(task api.PlayTask) []string {
			return []string{
				formatTaskStatus(task.Status),
				fmt.Sprint(task.Init),
				fmt.Sprint(task.Helper),
			}
		}

		return labcli.NewMapTablePrinter[api.PlayTask](w, header, rowFunc, true)
	case "json":
		return labcli.NewJSONPrinter[api.PlayTask, map[string]api.PlayTask](w)
	case "name":
		return labcli.NewMapKeyPrinter[api.PlayTask](w)
	default:
		// This should never happen
		panic(fmt.Errorf("invalid output format: %s (supported formats: table, json, name)", output))
	}
}

func formatTaskStatus(status api.PlayTaskStatus) string {
	switch status {
	case api.PlayTaskStatusNone:
		return "none"
	case api.PlayTaskStatusCreated:
		return "created"
	case api.PlayTaskStatusBlocked:
		return "blocked"
	case api.PlayTaskStatusRunning:
		return "running"
	case api.PlayTaskStatusFailed:
		return "failed"
	case api.PlayTaskStatusCompleted:
		return "completed"
	default:
		return "unknown"
	}
}

func taskIsFinished(task api.PlayTask) bool {
	return task.Status == api.PlayTaskStatusCompleted || task.Status == api.PlayTaskStatusFailed
}
