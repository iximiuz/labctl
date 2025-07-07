package tutorial

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

const startTutorialTimeout = 10 * time.Minute

type startOptions struct {
	tutorial string
	machine  string
	user     string

	noOpen    bool
	noSSH     bool
	keepAlive bool

	ide string

	safetyDisclaimerConsent bool

	forwardAgent bool

	asFreeTierUser bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <tutorial-url|tutorial-name>",
		Short: `Learn with a tutorial from the comfort of your local command line`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"tutorial name is required\n\nHint: Use `labctl content list --kind tutorial` to see all available tutorials",
				)
			}

			opts.tutorial = args[0]
			if strings.HasPrefix(opts.tutorial, "https://") {
				parts := strings.Split(strings.Trim(opts.tutorial, "/"), "/")
				opts.tutorial = parts[len(parts)-1]
			}

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

			return labcli.WrapStatusError(runStartTutorial(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		`SSH into the machine with the given name (requires --ssh flag, default to the first machine)`,
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
		`Don't open the tutorial in the browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the tutorial playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.keepAlive,
		"keep-alive",
		false,
		`Keep the tutorial playground alive after exiting SSH session`,
	)
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		`Open the tutorial playground in the IDE by specifying the IDE name (supported: "code", "cursor", "windsurf")`,
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
	flags.BoolVar(
		&opts.asFreeTierUser,
		"as-free-tier-user",
		false,
		`Run this playground as a free tier user (handy for testing that the playground works on all tiers)`,
	)

	return cmd
}

type tutorialEvent string

const (
	EventTutorialReady tutorialEvent = "tutorial-ready"
	EventSSHConnEnded  tutorialEvent = "ssh-conn-ended"
	EventWSConnFailed  tutorialEvent = "ws-conn-failed"
)

func runStartTutorial(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showSafetyDisclaimerIfNeeded(ctx, opts.tutorial, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

	tut, err := cli.Client().StartTutorial(ctx, opts.tutorial, api.StartTutorialOptions{
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
		AsFreeTierUser:          opts.asFreeTierUser,
	})
	if err != nil {
		return fmt.Errorf("couldn't start the tutorial: %w", err)
	}

	if tut.Play == nil {
		return fmt.Errorf("tutorial doesn't have a playground associated with it")
	}

	if opts.machine == "" {
		opts.machine = tut.Play.Machines[0].Name
	} else {
		if tut.Play.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the tutorial playground", opts.machine)
		}
	}

	if opts.user == "" {
		if u := tut.Play.GetMachine(opts.machine).DefaultUser(); u != nil {
			opts.user = u.Name
		} else {
			opts.user = "root"
		}
	}
	if !tut.Play.GetMachine(opts.machine).HasUser(opts.user) {
		return fmt.Errorf("user %q not found in the machine %q", opts.user, opts.machine)
	}

	if !opts.noOpen {
		cli.PrintAux("Opening %s in your browser...\n", tut.PageURL)

		if err := open.Run(tut.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	playConn := api.NewPlayConn(ctx, tut.Play, cli.Client(), cli.Config().WebSocketOrigin())
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	eventCh := make(chan tutorialEvent, 100)
	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	go func() {
		if err := playConn.WaitPlayReady(startTutorialTimeout, spin); err != nil {
			slog.Debug("websocket connection failed", "error", err)

			eventCh <- EventWSConnFailed
			return
		}
		eventCh <- EventTutorialReady
	}()

	if opts.ide != "" {
		go func() {
			cli.PrintAux("Opening local IDE...\n")

			if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
				PlayID:  tut.Play.ID,
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
			case EventTutorialReady:
				if !opts.noSSH {
					cli.PrintAux("SSH-ing into tutorial playground (%s machine)...\n", opts.machine)

					var errCh <-chan error

					sess, errCh, err = ssh.StartSSHSession(ctx, cli, tut.Play.ID, opts.machine, opts.user, nil, opts.forwardAgent)
					if err != nil {
						return fmt.Errorf("couldn't start SSH session: %w", err)
					}

					go func() {
						if err := <-errCh; err != nil {
							slog.Debug("SSH session error: " + err.Error())
						}

						if err := sess.Wait(); err != nil {
							slog.Debug("SSH session wait said: " + err.Error())
						}
						eventCh <- EventSSHConnEnded
					}()
				} else {
					cli.PrintAux("Tutorial playground is ready!\n")
					cli.PrintAux("Tutorial page: %s\n", tut.PageURL)
					if opts.keepAlive {
						cli.PrintAux("The tutorial playground will be kept alive.\n")
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
					cli.PrintAux("The tutorial playground will be kept alive.\n")
					cli.PrintAux("You can access it at: %s\n", tut.PageURL)
				} else {
					cli.PrintAux("Stopping the playground...\n")

					if _, err := cli.Client().StopTutorial(ctx, tut.Name); err != nil {
						cli.PrintErr("Error stopping the tutorial: %v\n", err)
					} else {
						cli.PrintAux("Playground stopped.\r\n")
					}
				}
				return nil
			}
		}
	}
}

func showSafetyDisclaimerIfNeeded(
	ctx context.Context,
	tutName string,
	cli labcli.CLI,
	consent bool,
) (bool, error) {
	if consent {
		return true, nil
	}

	tut, err := cli.Client().GetTutorial(ctx, tutName)
	if err != nil {
		return false, fmt.Errorf("couldn't get tutorial %q: %w", tutName, err)
	}

	if tut.IsOfficial() {
		return true, nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user info: %w", err)
	}

	if tut.IsAuthoredBy(me.ID) {
		return true, nil
	}

	return safety.ShowSafetyDisclaimer(cli)
}
