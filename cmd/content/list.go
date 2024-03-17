package content

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	kind ContentKind
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:   "list [--kind challenge|tutorial|course]",
		Short: "List authored content, possibly filtered by kind.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.Var(
		&opts.kind,
		"kind",
		`Content kind to filter by - one of challenge, tutorial, course (an empty string means all)`,
	)

	return cmd
}

type AuthoredContent struct {
	Challenges []api.Challenge `json:"challenges" yaml:"challenges"`
	Tutorials  []api.Tutorial  `json:"tutorials" yaml:"tutorials"`
	// Courses    []api.Course    `json:"courses"    yaml:"courses"`
}

func runListContent(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	var content AuthoredContent

	if opts.kind == "" || opts.kind == KindChallenge {
		challenges, err := cli.Client().ListAuthoredChallenges(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored challenges: %w", err)
		}

		content.Challenges = challenges
	}

	if opts.kind == "" || opts.kind == KindTutorial {
		tutorials, err := cli.Client().ListAuthoredTutorials(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored tutorials: %w", err)
		}

		content.Tutorials = tutorials
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(content); err != nil {
		return err
	}

	return nil
}
