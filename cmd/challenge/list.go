package challenge

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	category string
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [--category <linux|containers|kubernetes|...>]",
		Aliases: []string{"ls"},
		Short:   "List challenges, optionally filtered by category",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListChallenges(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.category,
		"category",
		"",
		`Category to filter by - one of linux, containers, kubernetes, ... (an empty string means all)`,
	)

	return cmd
}

type challengeListItem struct {
	Name        string   `json:"name" yaml:"name"`
	Title       string   `json:"title" yaml:"title"`
	Description string   `json:"description" yaml:"description"`
	Categories  []string `json:"categories" yaml:"categories"`
	Tags        []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	URL         string   `json:"url" yaml:"url"`
	Attempted   int      `json:"attempted" yaml:"attempted"`
	Completed   int      `json:"completed" yaml:"completed"`
}

func runListChallenges(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	challenges, err := cli.Client().ListChallenges(ctx, &api.ListChallengesOptions{
		Category: opts.category,
	})
	if err != nil {
		return fmt.Errorf("cannot list challenges: %w", err)
	}

	var items []challengeListItem
	for _, ch := range challenges {
		items = append(items, challengeListItem{
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
