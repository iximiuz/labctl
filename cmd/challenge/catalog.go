package challenge

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/internal/api"
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
		Aliases: []string{"catalog"},
		Short:   "List challenges from the catalog, optionally filtered by category and/or status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runCatalogChallenges(cmd.Context(), cli, &opts))
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
		`Status to filter by; multiple --status flags can be used. Valid statuses: todo, attempted, solved`,
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

func runCatalogChallenges(ctx context.Context, cli labcli.CLI, opts *catalogOptions) error {
	challenges, err := cli.Client().ListChallenges(ctx, &api.ListChallengesOptions{
		Category: opts.category,
		Status:   opts.status,
	})
	if err != nil {
		return fmt.Errorf("cannot list challenges: %w", err)
	}

	var items []catalogItem
	for _, ch := range challenges {
		items = append(items, catalogItem{
			Name:        ch.Name,
			Title:       ch.Title,
			Description: ch.Description,
			Categories:  ch.Categories,
			Tags:        ch.Tags,
			URL:         ch.PageURL,
			Attempted:   ch.AttemptCount,
			Completed:   ch.CompletionCount,
		})
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(items); err != nil {
		return err
	}

	return nil
}
