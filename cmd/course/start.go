package course

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/safety"
	issh "github.com/iximiuz/labctl/internal/ssh"
)

const startCourseLessonTimeout = 10 * time.Minute

type startOptions struct {
	course string
	lesson string
	module string

	machine string
	user    string

	noOpen    bool
	noSSH     bool
	keepAlive bool

	ide string

	safetyDisclaimerConsent bool

	forwardAgent bool

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

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

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
	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		`SSH into the machine with the given name (default to the first machine)`,
	)
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		`SSH user (default: the machine's default login user)`,
	)
	flags.BoolVar(
		&opts.noOpen,
		"no-open",
		false,
		`Don't open the lesson in the browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the lesson playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.keepAlive,
		"keep-alive",
		false,
		`Keep the lesson playground alive after exiting SSH session`,
	)
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		`Open the lesson playground in the IDE by specifying the IDE name (supported: "code", "cursor", "windsurf")`,
	)
	flags.BoolVar(
		&opts.safetyDisclaimerConsent,
		"safety-disclaimer-consent",
		false,
		`Acknowledge the safety disclaimer`,
	)
	flags.BoolVar(
		&opts.forwardAgent,
		"forward-agent",
		false,
		`INSECURE: Forward the SSH agent to the playground VM (use at your own risk)`,
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

type courseLessonEvent string

const (
	EventLessonReady  courseLessonEvent = "lesson-ready"
	EventSSHConnEnded courseLessonEvent = "ssh-conn-ended"
	EventWSConnFailed courseLessonEvent = "ws-conn-failed"
)

func runStartCourseLesson(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showCourseSafetyDisclaimerIfNeeded(ctx, opts.course, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

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
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
		AsFreeTierUser:          opts.asFreeTierUser,
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

	cli.PrintOut("%s\n", playID)

	// Resolve machine and user.
	if opts.machine == "" {
		opts.machine = play.Machines[0].Name
	} else {
		if play.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the lesson playground", opts.machine)
		}
	}

	if opts.user == "" {
		if u := play.GetMachine(opts.machine).DefaultUser(); u != nil {
			opts.user = u.Name
		} else {
			opts.user = "root"
		}
	}
	if !play.GetMachine(opts.machine).HasUser(opts.user) {
		return fmt.Errorf("user %q not found in the machine %q", opts.user, opts.machine)
	}

	// Open browser.
	if !opts.noOpen {
		cli.PrintAux("Opening %s in your browser...\n", play.PageURL)

		if err := open.Run(play.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	// Event-driven flow with WebSocket + SSH + IDE.
	playConn := api.NewPlayConn(ctx, play, cli.Client(), cli.Config().WebSocketOrigin())
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	eventCh := make(chan courseLessonEvent, 100)
	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	go func() {
		if err := playConn.WaitPlayReady(startCourseLessonTimeout, spin); err != nil {
			slog.Debug("websocket connection failed", "error", err)

			eventCh <- EventWSConnFailed
			return
		}
		eventCh <- EventLessonReady
	}()

	if opts.ide != "" {
		go func() {
			cli.PrintAux("Opening local IDE...\n")

			if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
				PlayID:  play.ID,
				Machine: opts.machine,
				User:    opts.user,
				IDE:     opts.ide,
			}); err != nil {
				cli.PrintErr("Error running IDE session: %v\n", err)
			}
		}()
	}

	var sess *issh.Session

	for {
		select {
		case <-ctx.Done():
			return nil

		case ev := <-eventCh:
			switch ev {
			case EventLessonReady:
				if !opts.noSSH {
					cli.PrintAux("SSH-ing into lesson playground (%s machine)...\n", opts.machine)

					var errCh <-chan error

					sess, errCh, err = ssh.StartSSHSession(ctx, cli, play, opts.machine, opts.user, nil, opts.forwardAgent)
					if err != nil {
						return fmt.Errorf("couldn't start SSH session: %w", err)
					}

					go func() {
						if err := <-errCh; err != nil {
							slog.Debug("SSH session error: " + err.Error())
						}
					}()

					go func() {
						if err := sess.Wait(); err != nil {
							slog.Debug("SSH session wait said: " + err.Error())
						}
						eventCh <- EventSSHConnEnded
					}()
				} else {
					cli.PrintAux("Lesson playground is ready!\n")
					cli.PrintAux("Lesson page: %s\n", play.PageURL)
					if opts.keepAlive {
						cli.PrintAux("The lesson playground will be kept alive.\n")
						cli.PrintAux("Press ENTER to continue...\n")
						var input string
						fmt.Scanln(&input)
					}
					return nil
				}

			case EventWSConnFailed:
				return fmt.Errorf("play connection failed")

			case EventSSHConnEnded:
				cli.PrintAux("\r\n")
				if opts.keepAlive {
					cli.PrintAux("The lesson playground will be kept alive.\n")
					cli.PrintAux("You can access it at: %s\n", play.PageURL)
				} else {
					cli.PrintAux("Stopping the playground...\n")

					if _, err := cli.Client().StopCourseLesson(ctx, opts.course, moduleName, lessonName); err != nil {
						cli.PrintErr("Error stopping the lesson: %v\n", err)
					} else {
						cli.PrintAux("Playground stopped.\r\n")
					}
				}
				return nil
			}
		}
	}
}

func showCourseSafetyDisclaimerIfNeeded(
	ctx context.Context,
	courseName string,
	cli labcli.CLI,
	consent bool,
) (bool, error) {
	if consent {
		return true, nil
	}

	course, err := cli.Client().GetCourse(ctx, courseName)
	if err != nil {
		return false, fmt.Errorf("couldn't get course %q: %w", courseName, err)
	}

	if course.IsOfficial() {
		return true, nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user info: %w", err)
	}

	if course.IsAuthoredBy(me.ID) {
		return true, nil
	}

	return safety.ShowSafetyDisclaimer(cli)
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
