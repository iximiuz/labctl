package playground

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type statusOptions struct {
	output string
}

func (opts *statusOptions) validate() error {
	if opts.output != "table" && opts.output != "json" && opts.output != "yaml" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json, yaml)", opts.output)
	}
	return nil
}

func newStatusCommand(cli labcli.CLI) *cobra.Command {
	var opts statusOptions

	cmd := &cobra.Command{
		Use:               "status [flags] <play-id>",
		Short:             "Show the status of a playground session",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.NonDestroyedPlays(cli),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runStatus(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.output,
		"output",
		"o",
		"table",
		"Output format: table, json, yaml",
	)

	return cmd
}

// playStatusView is the curated, output-friendly view of a playground session.
type playStatusView struct {
	ID         string `json:"id" yaml:"id"`
	Playground string `json:"playground" yaml:"playground"`
	Title      string `json:"title,omitempty" yaml:"title,omitempty"`
	State      string `json:"state" yaml:"state"`
	Active     bool   `json:"active" yaml:"active"`
	CreatedAt  string `json:"createdAt,omitempty" yaml:"createdAt,omitempty"`
	UpdatedAt  string `json:"updatedAt,omitempty" yaml:"updatedAt,omitempty"`
	ExpiresIn  int    `json:"expiresIn" yaml:"expiresIn"`
	PageURL    string `json:"pageUrl,omitempty" yaml:"pageUrl,omitempty"`

	Machines []machineStatusView `json:"machines,omitempty" yaml:"machines,omitempty"`

	Tasks taskStatusView `json:"tasks" yaml:"tasks"`
}

type machineStatusView struct {
	Name  string `json:"name" yaml:"name"`
	State string `json:"state" yaml:"state"`
}

type taskStatusView struct {
	Initialized   bool `json:"initialized" yaml:"initialized"`
	Failed        bool `json:"failed" yaml:"failed"`
	InitCompleted int  `json:"initCompleted" yaml:"initCompleted"`
	InitTotal     int  `json:"initTotal" yaml:"initTotal"`
	Completed     int  `json:"completed" yaml:"completed"`
	Total         int  `json:"total" yaml:"total"`
}

func newPlayStatusView(play *api.Play) playStatusView {
	machines := make([]machineStatusView, 0, len(play.Machines))
	for _, m := range play.Machines {
		machines = append(machines, machineStatusView{
			Name:  m.Name,
			State: string(play.MachineState(m.Name)),
		})
	}

	return playStatusView{
		ID:         play.ID,
		Playground: play.Playground.Name,
		Title:      play.Title,
		State:      string(play.State()),
		Active:     play.IsActive(),
		CreatedAt:  play.CreatedAt,
		UpdatedAt:  play.UpdatedAt,
		ExpiresIn:  play.ExpiresIn,
		PageURL:    play.PageURL,
		Machines:   machines,
		Tasks: taskStatusView{
			Initialized:   play.IsInitialized(),
			Failed:        play.HasFailedTask(),
			InitCompleted: play.CountCompletedInitTasks(),
			InitTotal:     play.CountInitTasks(),
			Completed:     play.CountCompletedTasks(),
			Total:         play.CountTasks(),
		},
	}
}

func runStatus(ctx context.Context, cli labcli.CLI, playID string, opts *statusOptions) error {
	play, err := cli.Client().GetPlay(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	view := newPlayStatusView(play)

	switch opts.output {
	case "json":
		enc := json.NewEncoder(cli.OutputStream())
		enc.SetIndent("", "  ")
		return enc.Encode(view)
	case "yaml":
		enc := yaml.NewEncoder(cli.OutputStream())
		enc.SetIndent(2)
		defer enc.Close()
		return enc.Encode(view)
	default:
		printStatusTable(cli.OutputStream(), view)
		return nil
	}
}

func printStatusTable(w io.Writer, view playStatusView) {
	state := view.State
	if state == "" {
		state = "UNKNOWN"
	}
	if view.State == string(api.StateRunning) && view.ExpiresIn > 0 {
		state = fmt.Sprintf("%s (expires %s)", state,
			humanize.Time(time.Now().Add(time.Duration(view.ExpiresIn)*time.Millisecond)))
	}

	var machines []string
	for _, m := range view.Machines {
		machines = append(machines, fmt.Sprintf("%s=%s", m.Name, m.State))
	}

	tasks := fmt.Sprintf("%d/%d completed", view.Tasks.Completed, view.Tasks.Total)
	if view.Tasks.Failed {
		tasks += " (some failed)"
	}
	initTasks := fmt.Sprintf("%d/%d completed", view.Tasks.InitCompleted, view.Tasks.InitTotal)

	fmt.Fprintf(w, "ID:          %s\n", view.ID)
	fmt.Fprintf(w, "Playground:  %s\n", view.Playground)
	if view.Title != "" {
		fmt.Fprintf(w, "Title:       %s\n", view.Title)
	}
	fmt.Fprintf(w, "State:       %s\n", state)
	if view.CreatedAt != "" {
		fmt.Fprintf(w, "Created:     %s\n", humanize.Time(safeParseTime(view.CreatedAt)))
	}
	fmt.Fprintf(w, "Init tasks:  %s\n", initTasks)
	fmt.Fprintf(w, "Tasks:       %s\n", tasks)
	if len(machines) > 0 {
		fmt.Fprintf(w, "Machines:    %s\n", strings.Join(machines, ", "))
	}
	if view.PageURL != "" {
		fmt.Fprintf(w, "Page URL:    %s\n", view.PageURL)
	}
}
