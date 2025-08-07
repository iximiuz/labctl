package playground

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type tasksOptions struct {
	output string
}

func (opts *tasksOptions) validate() error {
	if opts.output != "table" && opts.output != "json" && opts.output != "name" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json, name)", opts.output)
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
		"Output format: table, json, name",
	)

	return cmd
}

func runListTasks(ctx context.Context, cli labcli.CLI, playgroundID string, opts *tasksOptions) error {
	play, err := cli.Client().GetPlay(ctx, playgroundID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	printer := newPrinter(cli.OutputStream(), opts.output)

	err = printer.Print(play.Tasks)
	if err != nil {
		return err
	}
	defer printer.Flush()

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
