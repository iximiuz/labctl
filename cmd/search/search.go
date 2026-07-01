package search

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/labcli"
)

// indexableKinds is the set of content kinds the search endpoint accepts, kept
// in sync with sherlock/indexable.js (SHERLOCK_INDEXABLE_KINDS).
var indexableKinds = []string{
	"challenge",
	"course",
	"doc",
	"lesson",
	"playground",
	"roadmap",
	"skill-path",
	"tutorial",
	"vendor",
}

type outputFormat string

const (
	outputPretty outputFormat = "pretty"
	outputJSON   outputFormat = "json"
	outputYAML   outputFormat = "yaml"
)

type options struct {
	kinds        []string
	categories   []string
	tags         []string
	difficulties []string
	limit        int
	offset       int
	output       string
	quiet        bool
	open         bool
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:     "search [flags] <query>...",
		Aliases: []string{"s", "find"},
		Short:   "Search iximiuz Labs content - challenges, tutorials, courses, and more",
		Long: `Search across all iximiuz Labs content using free-text queries and facet filters.

Results are relevance-ranked when a query is given. You can narrow them down by
kind, category, tag, and difficulty. Pass --open to pick a result and jump
straight to it in your browser.`,
		Example: `  # Free-text search across everything
  labctl search kubernetes networking

  # Only challenges and tutorials, filtered by category
  labctl search --kind challenge --kind tutorial --category linux "namespaces"

  # Browse a single kind without a query
  labctl search --kind challenge --category kubernetes

  # Search and open a result in the browser
  labctl search --open docker`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runSearch(cmd.Context(), cli, &opts, strings.Join(args, " ")))
		},
	}

	flags := cmd.Flags()

	flags.StringSliceVarP(
		&opts.kinds,
		"kind",
		"k",
		nil,
		fmt.Sprintf("Content kind(s) to search; repeatable. One of: %s", strings.Join(indexableKinds, ", ")),
	)
	flags.StringSliceVar(
		&opts.categories,
		"category",
		nil,
		"Category to filter by; repeatable (e.g. linux, containers, kubernetes, networking)",
	)
	flags.StringSliceVar(
		&opts.tags,
		"tag",
		nil,
		"Tag to filter by; repeatable",
	)
	flags.StringSliceVar(
		&opts.difficulties,
		"difficulty",
		nil,
		"Difficulty to filter by; repeatable (e.g. easy, medium, hard)",
	)
	flags.IntVarP(
		&opts.limit,
		"limit",
		"n",
		20,
		"Maximum number of results to show (max 100)",
	)
	flags.IntVar(
		&opts.offset,
		"offset",
		0,
		"Number of results to skip (for paging through matches)",
	)
	flags.StringVarP(
		&opts.output,
		"output",
		"o",
		"pretty",
		`Output format - one of "pretty", "json", or "yaml"`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		"Only print content names (kind/name), one per line",
	)
	flags.BoolVar(
		&opts.open,
		"open",
		false,
		"Interactively pick a result and open it in the browser",
	)

	return cmd
}

func runSearch(ctx context.Context, cli labcli.CLI, opts *options, search string) error {
	if err := validateKinds(opts.kinds); err != nil {
		return err
	}

	// The endpoint refuses an unbounded full-catalog gather: without a query it
	// needs at least one kind to know what to browse. Fail fast with guidance
	// rather than silently returning nothing.
	if search == "" && len(opts.kinds) == 0 {
		return labcli.NewStatusError(1,
			"nothing to search for - provide a query (e.g. `labctl search kubernetes`) or a --kind to browse")
	}

	result, err := cli.Client().Search(ctx, api.SearchOptions{
		Search:       search,
		Kinds:        opts.kinds,
		Categories:   opts.categories,
		Tags:         opts.tags,
		Difficulties: opts.difficulties,
		Limit:        opts.limit,
		Offset:       opts.offset,
	})
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	switch outputFormat(opts.output) {
	case outputJSON:
		return labcli.NewJSONPrinter[api.SearchItem, []api.SearchItem](cli.OutputStream()).Print(result.Items)
	case outputYAML:
		return labcli.NewYAMLPrinter[api.SearchItem, []api.SearchItem](cli.OutputStream()).Print(result.Items)
	case outputPretty:
		// handled below
	default:
		return labcli.NewStatusError(1, "unknown output format %q - use pretty, json, or yaml", opts.output)
	}

	if opts.quiet {
		for _, item := range result.Items {
			cli.PrintOut("%s/%s\n", item.Kind, item.Name)
		}
		return nil
	}

	renderResults(cli, search, opts, result)

	if opts.open {
		return openInteractive(cli, result.Items)
	}

	return nil
}

func validateKinds(kinds []string) error {
	for _, k := range kinds {
		if !slices.Contains(indexableKinds, k) {
			return labcli.NewStatusError(1,
				"unknown content kind %q - valid kinds: %s", k, strings.Join(indexableKinds, ", "))
		}
	}
	return nil
}

func openInteractive(cli labcli.CLI, items []api.SearchItem) error {
	if len(items) == 0 {
		return nil
	}
	if !cli.OutputStream().IsTerminal() {
		return labcli.NewStatusError(1, "--open requires an interactive terminal")
	}

	styler := newStyler(true)

	huhOpts := make([]huh.Option[string], 0, len(items))
	for _, item := range items {
		label := fmt.Sprintf("%s  %s", styler.kindBadge(item.Kind), item.Title)
		huhOpts = append(huhOpts, huh.NewOption(label, item.PageURL))
	}

	var pageURL string
	err := huh.NewSelect[string]().
		Title("Open a result in your browser").
		Options(huhOpts...).
		Value(&pageURL).
		Run()
	if err != nil {
		// User aborted (Esc/Ctrl-C) - nothing to open, not an error worth failing on.
		return nil
	}

	if pageURL != "" {
		browser.OpenWithFallbackMessage(cli, pageURL)
	}
	return nil
}
