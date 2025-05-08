package playground

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	all   bool
	quiet bool
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Aliases: []string{"ls"},
		Short:   `List current or recently run playgrounds (up to 50)`,
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

	return cmd
}

func runListPlays(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	printer := newListPrinter(cli.OutputStream(), opts.quiet)
	defer printer.flush()

	printer.printHeader()

	plays, err := cli.Client().ListPlays(ctx)
	if err != nil {
		return fmt.Errorf("couldn't list playgrounds: %w", err)
	}

	for _, play := range plays {
		if opts.all || play.Active {
			printer.printOne(play)
		}
	}

	return nil
}

type listPrinter struct {
	quiet  bool
	header []string
	writer *tabwriter.Writer
}

func newListPrinter(outStream io.Writer, quiet bool) *listPrinter {
	header := []string{
		"PLAYGROUND ID",
		"NAME",
		"CREATED",
		"STATUS",
		"LINK",
	}

	return &listPrinter{
		quiet:  quiet,
		header: header,
		writer: tabwriter.NewWriter(outStream, 0, 4, 2, ' ', 0),
	}
}

func (p *listPrinter) printHeader() {
	if !p.quiet {
		fmt.Fprintln(p.writer, strings.Join(p.header, "\t"))
	}
}

func (p *listPrinter) printOne(play *api.Play) {
	if p.quiet {
		fmt.Fprintln(p.writer, play.ID)
		return
	}

	var link string
	if play.Active || play.TutorialName+play.ChallengeName+play.CourseName != "" {
		link = play.PageURL
	}

	fields := []string{
		play.ID,
		play.Playground.Name,
		humanize.Time(safeParseTime(play.CreatedAt)),
		playStatus(play),
		link,
	}

	fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
}

func (p *listPrinter) flush() {
	p.writer.Flush()
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
