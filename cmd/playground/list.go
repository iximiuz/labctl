package playground

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	all    bool
	quiet  bool
	filter string
	output string
}

func (opts *listOptions) validate() error {
	if opts.output != "table" && opts.output != "json" {
		return fmt.Errorf("invalid output format: %s (supported formats: table, json)", opts.output)
	}
	return nil
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Aliases: []string{"ls"},
		Short:   `List current or recently run playgrounds (up to 50)`,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			return opts.validate()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListPlays(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.all,
		"all",
		"a",
		false,
		`List all playgrounds (including terminated)`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print playground IDs`,
	)
	flags.StringVarP(
		&opts.filter,
		"filter",
		"f",
		"",
		`Filter playgrounds by tutorial=<name>, challenge=<name>, course=<name>, or playground=<name>`,
	)
	flags.StringVarP(
		&opts.output,
		"output",
		"o",
		"table",
		`Output format: table, json`,
	)

	return cmd
}

func runListPlays(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	filter, err := parseFilter(opts.filter)
	if err != nil {
		return err
	}

	plays, err := cli.Client().ListPlays(ctx)
	if err != nil {
		return fmt.Errorf("couldn't list playgrounds: %w", err)
	}

	var filteredPlays []*api.Play
	for _, play := range plays {
		if (opts.all || play.Active) && filter.matches(play) {
			filteredPlays = append(filteredPlays, play)
		}
	}

	if opts.quiet {
		for _, play := range filteredPlays {
			fmt.Fprintln(cli.OutputStream(), play.ID)
		}
		return nil
	}

	printer := newListPrinter(cli.OutputStream(), opts.output)

	if err := printer.Print(filteredPlays); err != nil {
		return err
	}
	defer printer.Flush()

	return nil
}

type listPrinter interface {
	Print([]*api.Play) error
	Flush()
}

func newListPrinter(w io.Writer, output string) listPrinter {
	switch output {
	case "table":
		header := []string{
			"PLAYGROUND ID",
			"NAME",
			"CREATED",
			"STATUS",
			"LINK",
		}

		rowFunc := func(play *api.Play) []string {
			var link string
			if play.Active || play.TutorialName+play.ChallengeName+play.CourseName != "" {
				link = play.PageURL
			}

			return []string{
				play.ID,
				play.Playground.Name,
				humanize.Time(safeParseTime(play.CreatedAt)),
				playStatus(play),
				link,
			}
		}

		return labcli.NewSliceTablePrinter[*api.Play](w, header, rowFunc)
	case "json":
		return labcli.NewJSONPrinter[*api.Play, []*api.Play](w)
	// case "id":
	default:
		// This should never happen
		panic(fmt.Errorf("invalid output format: %s (supported formats: table, json)", output))
	}
}

func playStatus(play *api.Play) string {
	if play.Running {
		return fmt.Sprintf("running (expires in %s)",
			humanize.Time(time.Now().Add(time.Duration(play.ExpiresIn)*time.Millisecond)))
	}
	if play.Destroyed {
		return fmt.Sprintf("terminated %s",
			humanize.Time(safeParseTime(play.LastStateAt)))
	}
	if play.Failed {
		return fmt.Sprintf("failed %s",
			humanize.Time(safeParseTime(play.LastStateAt)))
	}

	return "unknown"
}

func safeParseTime(s string) time.Time {
	t, _ := time.Parse(time.RFC3339, s)
	return t
}

func parseFilter(filter string) (*playFilter, error) {
	if filter == "" {
		return &playFilter{}, nil
	}

	parts := strings.SplitN(filter, "=", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid filter format: %s (expected format: type=value)", filter)
	}

	filterType := strings.ToLower(parts[0])
	filterValue := parts[1]

	switch filterType {
	case "tutorial", "challenge", "course", "playground":
		return &playFilter{
			kind:  filterType,
			value: filterValue,
		}, nil
	default:
		return nil, fmt.Errorf("unknown filter type: %s (supported types: tutorial, challenge, course, playground)", filterType)
	}
}

type playFilter struct {
	kind  string
	value string
}

func (f *playFilter) matches(play *api.Play) bool {
	if f == nil || f.kind == "" {
		return true
	}

	switch f.kind {
	case "tutorial":
		return play.TutorialName == f.value
	case "challenge":
		return play.ChallengeName == f.value
	case "course":
		return play.CourseName == f.value
	case "playground":
		return play.Playground.Name == f.value
	default:
		return false
	}
}
