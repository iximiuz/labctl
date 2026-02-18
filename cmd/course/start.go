package course

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

const startCourseLessonTimeout = 10 * time.Minute

type startOptions struct {
	course string
	lesson string
	module string

	quiet bool

	asFreeTierUser bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <course-name> <lesson>",
		Short: `Start a course lesson`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.course = args[0]
			if strings.HasPrefix(opts.course, "https://") {
				parts := strings.Split(strings.Trim(opts.course, "/"), "/")
				opts.course = parts[len(parts)-1]
			}

			opts.lesson = args[1]

			return labcli.WrapStatusError(runStartCourseLesson(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.module,
		"module",
		"",
		`Module name or slug (needed only if the lesson slug is ambiguous across modules)`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print the playground ID`,
	)
	flags.BoolVar(
		&opts.asFreeTierUser,
		"as-free-tier-user",
		false,
		`Run this playground as a free tier user (handy for testing that the playground works on all tiers)`,
	)

	return cmd
}

func runStartCourseLesson(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	cli.PrintAux("Getting course %s...\n", opts.course)

	course, err := cli.Client().GetCourse(ctx, opts.course)
	if err != nil {
		return fmt.Errorf("couldn't get course: %w", err)
	}

	moduleName, lessonName, lesson, err := course.FindLesson(opts.module, opts.lesson)
	if err != nil {
		return err
	}

	if lesson.Playground == nil {
		return fmt.Errorf("lesson %q has no playground", lessonName)
	}

	cli.PrintAux("Starting lesson %s (module %s)...\n", lessonName, moduleName)

	course, err = cli.Client().StartCourseLesson(ctx, opts.course, moduleName, lessonName, api.StartCourseLessonOptions{
		AsFreeTierUser: opts.asFreeTierUser,
	})
	if err != nil {
		return fmt.Errorf("couldn't start the course lesson: %w", err)
	}

	playID, err := extractPlayID(course, moduleName, lessonName)
	if err != nil {
		return err
	}

	play, err := cli.Client().GetPlay(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if len(play.Tasks) > 0 {
		playConn := api.NewPlayConn(ctx, play, cli.Client(), cli.Config().WebSocketOrigin())
		if err := playConn.Start(); err != nil {
			return fmt.Errorf("couldn't start play connection: %w", err)
		}

		spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
		spin.Writer = cli.AuxStream()
		if err := playConn.WaitPlayReady(startCourseLessonTimeout, spin); err != nil {
			return fmt.Errorf("playground initialization failed: %w", err)
		}
	}

	cli.PrintOut("%s\n", playID)

	return nil
}

func extractPlayID(course *api.Course, moduleName, lessonName string) (string, error) {
	if course.Learning == nil {
		return "", fmt.Errorf("course has no learning state")
	}

	mod, ok := course.Learning.Modules[moduleName]
	if !ok {
		return "", fmt.Errorf("module %q not found in learning state", moduleName)
	}

	les, ok := mod.Lessons[lessonName]
	if !ok {
		return "", fmt.Errorf("lesson %q not found in learning state", lessonName)
	}

	if les.Play == "" {
		return "", fmt.Errorf("lesson %q has no active playground", lessonName)
	}

	return les.Play, nil
}
