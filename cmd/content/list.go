package content

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	kind content.ContentKind
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [--kind challenge|tutorial|skill-path|course|training]",
		Aliases: []string{"ls"},
		Short:   "List authored content, possibly filtered by kind.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runListContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.Var(
		&opts.kind,
		"kind",
		`Content kind to filter by - one of 'challenge', 'tutorial', 'skill-path', 'course', or 'training' (an empty string means all content types)`,
	)

	return cmd
}

type AuthoredContent struct {
	Challenges []api.Challenge `json:"challenges" yaml:"challenges"`
	Tutorials  []api.Tutorial  `json:"tutorials" yaml:"tutorials"`
	Roadmaps   []api.Roadmap   `json:"roadmaps" yaml:"roadmaps"`
	SkillPaths []api.SkillPath `json:"skill-paths" yaml:"skill-paths"`
	Courses    []api.Course    `json:"courses"    yaml:"courses"`
	Trainings  []api.Training  `json:"trainings"  yaml:"trainings"`
}

func runListContent(ctx context.Context, cli labcli.CLI, opts *listOptions) error {
	var authored AuthoredContent

	if opts.kind == "" || opts.kind == content.KindChallenge {
		challenges, err := cli.Client().ListAuthoredChallenges(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored challenges: %w", err)
		}

		authored.Challenges = challenges
	}

	if opts.kind == "" || opts.kind == content.KindTutorial {
		tutorials, err := cli.Client().ListAuthoredTutorials(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored tutorials: %w", err)
		}

		authored.Tutorials = tutorials
	}

	if opts.kind == "" || opts.kind == content.KindRoadmap {
		roadmaps, err := cli.Client().ListAuthoredRoadmaps(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored roadmaps: %w", err)
		}

		authored.Roadmaps = roadmaps
	}

	if opts.kind == "" || opts.kind == content.KindSkillPath {
		skillPaths, err := cli.Client().ListAuthoredSkillPaths(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored skill paths: %w", err)
		}

		authored.SkillPaths = skillPaths
	}

	if opts.kind == "" || opts.kind == content.KindCourse {
		courses, err := cli.Client().ListAuthoredCourses(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored courses: %w", err)
		}

		authored.Courses = courses
	}

	if opts.kind == "" || opts.kind == content.KindTraining {
		trainings, err := cli.Client().ListAuthoredTrainings(ctx)
		if err != nil {
			return fmt.Errorf("cannot list authored trainings: %w", err)
		}

		authored.Trainings = trainings
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(authored); err != nil {
		return err
	}

	return nil
}
