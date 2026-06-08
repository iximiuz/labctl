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
	"github.com/iximiuz/labctl/internal/completion"
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
	if opts.output != "table" && opts.output != "json" && opts.output != "yaml" && opts.output != "name" && opts.output != "none" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json, yaml, name, none)", opts.output)
	}

	if opts.kind != "" && opts.kind != "init" && opts.kind != "helper" && opts.kind != "regular" {
		return fmt.Errorf("invalid kind: %s (supported kinds: init, helper, regular)", opts.kind)
	}

	return nil
}

func newTasksCommand(cli labcli.CLI) *cobra.Command {
	var opts tasksOptions

	cmd := &cobra.Command{
		Use:               "tasks <play-id>",
		Short:             "List tasks of a playground session",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.NonDestroyedPlays(cli),
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
		"Output format: table, json, yaml, name, none",
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

	var waitErr error
	if opts.wait {
		b := backoff.NewExponentialBackOff()
		b.Multiplier = 1.3
		b.MaxInterval = 10 * time.Second

		_, err := backoff.Retry(
			ctx,
			operation,
			backoff.WithMaxElapsedTime(opts.timeout),
			backoff.WithBackOff(b),
		)
		spin.Stop()
		if err != nil && !errors.Is(err, errFailed) && !errors.Is(err, errUnfinished) {
			return err
		}
		waitErr = err
	}

	if opts.output != "none" {
		// The merged control-plane + data-plane view, with full task details for
		// privileged callers (super-admins, capability holders, authors).
		tasks, err := cli.Client().GetPlayTasks(ctx, playgroundID, nil)
		if err != nil {
			return fmt.Errorf("couldn't list playground tasks: %w", err)
		}

		byName := make(map[string]api.PlayTaskDetails, len(tasks))
		for _, task := range tasks {
			byName[task.Name] = task
		}

		printer := newTaskListPrinter(cli.OutputStream(), opts.output)

		if err := printer.Print(filterTasksByKind(byName, opts.kind)); err != nil {
			return err
		}
		printer.Flush()
	}

	if waitErr != nil {
		return labcli.NewStatusError(2, "%s", waitErr.Error())
	}

	return nil
}

type taskListPrinter interface {
	Print(map[string]api.PlayTaskDetails) error
	Flush()
}

func newTaskListPrinter(w io.Writer, output string) taskListPrinter {
	switch output {
	case "table":
		header := []string{
			"NAME",
			"MACHINE",
			"STATUS",
			"INIT",
			"HELPER",
		}

		rowFunc := func(task api.PlayTaskDetails) []string {
			return []string{
				task.Machine,
				formatTaskStatus(task.Status),
				fmt.Sprint(task.Init),
				fmt.Sprint(task.Helper),
			}
		}

		return labcli.NewMapTablePrinter[api.PlayTaskDetails](w, header, rowFunc, true)
	case "json":
		return labcli.NewJSONPrinter[api.PlayTaskDetails, map[string]api.PlayTaskDetails](w)
	case "yaml":
		return labcli.NewYAMLPrinter[api.PlayTaskDetails, map[string]api.PlayTaskDetails](w)
	case "name":
		return labcli.NewMapKeyPrinter[api.PlayTaskDetails](w)
	default:
		// This should never happen
		panic(fmt.Errorf("invalid output format: %s (supported formats: table, json, yaml, name)", output))
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

func filterTasksByKind(tasks map[string]api.PlayTaskDetails, kind string) map[string]api.PlayTaskDetails {
	if kind == "" {
		return tasks
	}

	filtered := make(map[string]api.PlayTaskDetails)
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
