package tutorial

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type catalogOptions struct {
	category []string
	status   []string
}

func newCatalogCommand(cli labcli.CLI) *cobra.Command {
	var opts catalogOptions

	cmd := &cobra.Command{
		Use:     "catalog",
		Aliases: []string{"cat"},
		Short:   "List tutorials from the catalog, optionally filtered by category and/or status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runCatalogTutorials(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringSliceVar(
		&opts.category,
		"category",
		[]string{},
		`Category to filter by; multiple --category flags can be used. Valid categories: linux, containers, kubernetes, networking, programming, observability, security, ci-cd`,
	)

	flags.StringSliceVar(
		&opts.status,
		"status",
		[]string{},
		`Status to filter by; multiple --status flags can be used. Valid statuses: todo, attempted, completed`,
	)

	return cmd
}

type catalogItem struct {
	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	URL         string   `json:"url" yaml:"url"`
	Attempted   int      `json:"attempted" yaml:"attempted"`
	Completed   int      `json:"completed" yaml:"completed"`
}

func runCatalogTutorials(ctx context.Context, cli labcli.CLI, opts *catalogOptions) error {
	tutorials, err := cli.Client().ListTutorials(ctx, &api.ListTutorialsOptions{
		Category: opts.category,
		Status:   opts.status,
	})
	if err != nil {
		return fmt.Errorf("cannot list tutorials: %w", err)
	}

	var items []catalogItem
	for _, tut := range tutorials {
		items = append(items, catalogItem{
			Name:        tut.Name,
			Title:       tut.Title,
			Description: tut.Description,
			Categories:  tut.Categories,
			Tags:        tut.Tags,
			URL:         tut.PageURL,
			Attempted:   tut.AttemptCount,
			Completed:   tut.CompletionCount,
		})
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(items); err != nil {
		return err
	}

	return nil
}
