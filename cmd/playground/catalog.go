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
		Short:   "List playgrounds from the catalog, optionally filtering by type",
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runCatalog(cmd.Context(), cli, &opts))
		},
	}
	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print playground names`,
	)
	flags.StringVarP(
		&opts.filter,
		"filter",
		"f",
		"",
		`Filter to use for catalog list. ("recent" | "popular" | "my-custom") (default no filter, meaning all)`,
	)
	return cmd
}

func runCatalog(ctx context.Context, cli labcli.CLI, opts *catalogOptions) error {
	playgrounds, err := cli.Client().ListPlaygrounds(ctx, &api.ListPlaygroundsOptions{
		Filter: opts.filter,
	})
	if err != nil {
		return fmt.Errorf("couldn't list playgrounds: %w", err)
	}

	slices.SortFunc(playgrounds, func(a, b api.Playground) int { return cmp.Compare(a.Name, b.Name) })

	cli.PrintAux("Available playgrounds:\n")

	if opts.quiet {
		for _, p := range playgrounds {
			cli.PrintOut("%s\n", p.Name)
		}
	} else {
		for _, p := range playgrounds {
			cli.PrintAux("  - %s - %s\n", p.Name, p.Description)
		}
	}

	return nil
}
