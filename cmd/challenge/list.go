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

func runListChallenges(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	challenges, err := cli.Client().ListChallenges(ctx, &api.ListChallengesOptions{
		Category: opts.category,
	})
	if err != nil {
		return fmt.Errorf("cannot list challenges: %w", err)
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(challenges); err != nil {
		return err
	}

	return nil
}
