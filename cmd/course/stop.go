package course

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const stopCourseLessonTimeout = 5 * time.Minute

type stopOptions struct {
	course string
	lesson string
	module string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:   "stop [flags] <course-name> <lesson>",
		Short: `Stop a running course lesson`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.course = args[0]
			if strings.HasPrefix(opts.course, "https://") {
				parts := strings.Split(strings.Trim(opts.course, "/"), "/")
				opts.course = parts[len(parts)-1]
			}

			opts.lesson = args[1]

			return labcli.WrapStatusError(runStopCourseLesson(cmd.Context(), cli, &opts))
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
		`Do not print any diagnostic messages`,
	)

	return cmd
}

func runStopCourseLesson(ctx context.Context, cli labcli.CLI, opts *stopOptions) error {
	cli.PrintAux("Getting course %s...\n", opts.course)

	course, err := cli.Client().GetCourse(ctx, opts.course)
	if err != nil {
		return fmt.Errorf("couldn't get course: %w", err)
	}

	moduleName, lessonName, _, err := course.FindLesson(opts.module, opts.lesson)
	if err != nil {
		return err
	}

	playID, err := extractPlayID(course, moduleName, lessonName)
	if err != nil {
		cli.PrintAux("Lesson is not running - nothing to stop.\n")
		return nil
	}

	play, err := cli.Client().GetPlay(ctx, playID)
	if err != nil {
		cli.PrintAux("Lesson is not running - nothing to stop.\n")
		return nil
	}

	if !play.IsActive() {
		cli.PrintAux("Lesson is not running - nothing to stop.\n")
		return nil
	}

	cli.PrintAux("Stopping lesson %s (module %s)...\n", lessonName, moduleName)

	if _, err := cli.Client().StopCourseLesson(ctx, opts.course, moduleName, lessonName); err != nil {
		return fmt.Errorf("couldn't stop the course lesson: %w", err)
	}

	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for playground to stop... "
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, stopCourseLessonTimeout)
	defer cancel()

	for ctx.Err() == nil {
		if play, err := cli.Client().GetPlay(ctx, playID); err == nil && !play.IsActive() {
			s.FinalMSG = "Waiting for playground to stop... Done.\n"
			s.Stop()

			return nil
		}

		time.Sleep(2 * time.Second)
	}

	s.Stop()

	cli.PrintAux("Playground has been stopped.\n")

	return nil
}
