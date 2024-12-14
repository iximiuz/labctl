package playground

import (
	"cmp"
	"context"
	"fmt"
	"slices"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type catalogOptions struct {
	filter string
	quiet  bool
}

func newCatalogCommand(cli labcli.CLI) *cobra.Command {
	var opts catalogOptions

	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"cat"},
		Short:   `List catalog`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListCatalogs(cmd.Context(), cli, &opts))
		},
	}
	flags := cmd.Flags()

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
		`Filter to use for catalog list. ("recent" | "popular" | "my-custom") (default "")`,
	)
	return cmd
}

func runListCatalogs(ctx context.Context, cli labcli.CLI, opts *catalogOptions) error {

	var catalog []api.Playground
	var err error

	// once the backend is fixed there is no need for this if
	if opts.filter != "" {
		catalog, err = cli.Client().ListPlaygrounds(ctx, opts.filter)
		if err != nil {
			return fmt.Errorf("couldn't list catalog: %w", err)
		}
	} else {
		catalog, err = cli.Client().ListPlaygrounds(ctx, "")
		if err != nil {
			return fmt.Errorf("couldn't list catalog: %w", err)
		}
		customs, err := cli.Client().ListPlaygrounds(ctx, "my-custom")
		if err != nil {
			return fmt.Errorf("couldn't list catalog: %w", err)
		}
		catalog = append(catalog, customs...)
	}

	slices.SortFunc(catalog, func(a, b api.Playground) int { return cmp.Compare(a.Name, b.Name) })

	fmt.Fprintln(cli.OutputStream(), "Available playgrounds:")
	if opts.quiet {
		for _, p := range catalog {
			fmt.Fprintln(cli.OutputStream(), p.Name)
		}

	} else {
		for _, p := range catalog {
			//fmt.Fprintf(cli.OutputStream(), "%-30v %-30v\n", p.Name, p.Title)
			fmt.Fprintf(cli.OutputStream(), "  - %s - %s\n", p.Name, p.Description)
		}
	}

	return nil
}
