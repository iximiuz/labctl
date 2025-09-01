package playground

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/briandowns/spinner"
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
	kind     string
}

func (opts *tasksOptions) validate() error {
	if opts.output != "table" && opts.output != "json" && opts.output != "name" && opts.output != "none" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json, name, none)", opts.output)
	}

	if opts.kind != "" && opts.kind != "init" && opts.kind != "helper" && opts.kind != "regular" {
		return fmt.Errorf("invalid kind: %s (supported kinds: init, helper, regular)", opts.kind)
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
		-1,
		"Timeout to wait for tasks to finish",
	)

	flags.StringVar(
		&opts.kind,
		"kind",
		"",
		"Filter tasks by kind: init, helper, regular",
	)

	return cmd
}

func runListTasks(ctx context.Context, cli labcli.CLI, playgroundID string, opts *tasksOptions) error {
	var (
		errFailed     = errors.New("some tasks failed")
		errUnfinished = errors.New("timed out waiting for tasks to finish")
	)

	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	operation := func() (*api.Play, error) {
		play, err := cli.Client().GetPlay(ctx, playgroundID)
		if err != nil {
			return nil, backoff.Permanent(fmt.Errorf("couldn't get playground: %w", err))
		}

		if !play.Active {
			return play, backoff.Permanent(errors.New("play has been terminated"))
		}

		if !opts.wait {
			return play, nil
		}

		if opts.kind == "helper" {
			return play, nil
		}

		if opts.kind == "init" && play.IsInitialized() {
			return play, nil
		}

		if play.IsInitialized() {
			spin.Prefix = fmt.Sprintf(
				"Waiting for tasks to complete: %d/%d ",
				play.CountCompletedTasks(), play.CountTasks(),
			)
		} else {
			spin.Prefix = fmt.Sprintf(
				"Warming up playground... Init tasks completed: %d/%d ",
				play.CountCompletedInitTasks(), play.CountInitTasks(),
			)
		}

		spin.Start()

		var failed, unfinished bool

		for _, task := range play.Tasks {
			if task.Helper {
				continue
			}

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

	b := backoff.NewExponentialBackOff()
	b.Multiplier = 1.3
	b.MaxInterval = 10 * time.Second

	play, err := backoff.Retry(
		ctx,
		operation,
		backoff.WithMaxElapsedTime(opts.timeout),
		backoff.WithBackOff(b),
	)
	spin.Stop()
	if err != nil && !errors.Is(err, errFailed) && !errors.Is(err, errUnfinished) {
		return err
	}

	if opts.output != "none" {
		printer := newTaskListPrinter(cli.OutputStream(), opts.output)

		filteredTasks := filterTasksByKind(play.Tasks, opts.kind)

		if err := printer.Print(filteredTasks); err != nil {
			return err
		}
		defer printer.Flush()
	}

	if err != nil {
		return labcli.NewStatusError(2, err.Error())
	}

	return nil
}

type taskListPrinter interface {
	Print(map[string]api.PlayTask) error
	Flush()
}

func newTaskListPrinter(w io.Writer, output string) taskListPrinter {
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

func filterTasksByKind(tasks map[string]api.PlayTask, kind string) map[string]api.PlayTask {
	if kind == "" {
		return tasks
	}

	filtered := make(map[string]api.PlayTask)
	for name, task := range tasks {
		switch kind {
		case "init":
			if task.Init {
				filtered[name] = task
			}
		case "helper":
			if task.Helper {
				filtered[name] = task
			}
		case "regular":
			if !task.Init && !task.Helper {
				filtered[name] = task
			}
		}
	}

	return filtered
}
