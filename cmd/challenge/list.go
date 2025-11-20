package challenge

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	quiet bool
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List running challenge attempts",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListChallenges(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print challenge names`,
	)

	return cmd
}

func runListChallenges(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	plays, err := cli.Client().ListPlays(ctx)
	if err != nil {
		return fmt.Errorf("cannot list plays: %w", err)
	}

	var challenges []*api.Challenge
	for _, play := range plays {
		if !play.IsActive() || play.ChallengeName == "" {
			continue
		}

		chal, err := cli.Client().GetChallenge(ctx, play.ChallengeName)
		if err != nil {
			return fmt.Errorf("cannot get challenge %s: %w", play.ChallengeName, err)
		}
		challenges = append(challenges, chal)
	}

	printer := newListPrinter(cli.OutputStream(), opts.quiet)
	defer printer.flush()

	printer.printHeader()

	for _, chal := range challenges {
		printer.printOne(chal)
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
		"TITLE",
		"URL",
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

func (p *listPrinter) printOne(chal *api.Challenge) {
	if p.quiet {
		fmt.Fprintln(p.writer, chal.Name)
		return
	}

	fields := []string{
		chal.Title,
		chal.PageURL,
	}

	fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
}

func (p *listPrinter) flush() {
	p.writer.Flush()
}
