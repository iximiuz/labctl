package challenge

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

const startChallengeTimeout = 10 * time.Minute

type startOptions struct {
	challenge string
	machine   string
	user      string

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
		Use:     "start [flags] <challenge-url|challenge-name>",
		Short:   `Solve a challenge from the comfort of your local command line`,
		Aliases: []string{"solve"},
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"challenge name is required\n\nHint: Use `labctl challenge catalog` to see all available challenges",
				)
			}

			opts.challenge = args[0]
			if strings.HasPrefix(opts.challenge, "https://") {
				parts := strings.Split(strings.Trim(opts.challenge, "/"), "/")
				opts.challenge = parts[len(parts)-1]
			}

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

			return labcli.WrapStatusError(runStartChallenge(cmd.Context(), cli, &opts))
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
		`Don't open the challenge in the browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the challenge playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.keepAlive,
		"keep-alive",
		false,
		`Keep the challenge playground alive after it's completed`,
	)
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		`Open the challenge playground in the IDE by specifying the IDE name (supported: "code", "cursor", "windsurf")`,
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

type challengeEvent string

const (
	EventChallengeReady       challengeEvent = "challenge-ready"
	EventChallengeCompletable challengeEvent = "challenge-completable"
	EventChallengeCompleted   challengeEvent = "challenge-completed"
	EventChallengeFailed      challengeEvent = "challenge-failed"
	EventSSHConnEnded         challengeEvent = "ssh-conn-ended"
	EventWSConnFailed         challengeEvent = "ws-conn-failed"
)

func runStartChallenge(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showSafetyDisclaimerIfNeeded(ctx, opts.challenge, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

	chal, err := cli.Client().StartChallenge(ctx, opts.challenge, api.StartChallengeOptions{
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
		AsFreeTierUser:          opts.asFreeTierUser,
	})
	if err != nil {
		return fmt.Errorf("couldn't start solving the challenge: %w", err)
	}

	if opts.machine == "" {
		opts.machine = chal.Play.Machines[0].Name
	} else {
		if chal.Play.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the challenge playground", opts.machine)
		}
	}

	if opts.user == "" {
		if u := chal.Play.GetMachine(opts.machine).DefaultUser(); u != nil {
			opts.user = u.Name
		} else {
			opts.user = "root"
		}
	}
	if !chal.Play.GetMachine(opts.machine).HasUser(opts.user) {
		return fmt.Errorf("user %q not found in the machine %q", opts.user, opts.machine)
	}

	if !opts.noOpen {
		cli.PrintAux("Opening %s in your browser...\n", chal.PageURL)

		if err := open.Run(chal.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	playConn := api.NewPlayConn(ctx, chal.Play, cli.Client(), cli.Config().WebSocketOrigin())
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	eventCh := make(chan challengeEvent, 100)
	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	go func() {
		if err := playConn.WaitPlayReady(startChallengeTimeout, spin); err != nil {
			eventCh <- EventWSConnFailed
			return
		}
		eventCh <- EventChallengeReady

		if err := playConn.WaitDone(); err != nil {
			eventCh <- EventWSConnFailed
			return
		}

		if chal.Play.IsCompletable() {
			eventCh <- EventChallengeCompletable
		} else {
			eventCh <- EventChallengeFailed
		}
	}()

	if opts.ide != "" {
		go func() {
			cli.PrintAux("Opening local IDE...\n")

			if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
				PlayID:  chal.Play.ID,
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
			case EventChallengeReady:
				if !opts.noSSH {
					cli.PrintAux("SSH-ing into challenge playground (%s machine)...\n", opts.machine)

					var errCh <-chan error

					sess, errCh, err = ssh.StartSSHSession(ctx, cli, chal.Play.ID, opts.machine, opts.user, nil, opts.forwardAgent)
					if err != nil {
						return fmt.Errorf("couldn't start SSH session: %w", err) // critical error
					}

					go func() {
						if err := <-errCh; err != nil {
							slog.Debug("SSH session error: " + err.Error())
						}

						if err := sess.Wait(); err != nil {
							slog.Debug("SSH session said: " + err.Error())
						}
						eventCh <- EventSSHConnEnded
					}()
				} else {
					cli.PrintAux("Challenge playground is ready!\n")
					cli.PrintAux("Challenge page: %s\n", chal.PageURL)
					if opts.keepAlive {
						cli.PrintAux("The challenge playground will be kept alive.\n")
						cli.PrintAux("Press ENTER to continue...\n")
						var input string
						fmt.Scanln(&input)
					}

					return nil
				}

			case EventChallengeCompletable:
				if _, err := cli.Client().CompleteChallenge(ctx, chal.Name); err != nil {
					slog.Debug("Error completing the challenge: " + err.Error())
					go func() {
						time.Sleep(5 * time.Second) // retry in 5 seconds without blocking the event loop
						eventCh <- EventChallengeCompletable
					}()
				} else {
					// cli.PrintAux("\033c\r") // Reset terminal
					cli.PrintAux("\r\n\r\n")
					cli.PrintAux("**********************************\r\n")
					cli.PrintAux("** Yay! Challenge completed! 🎉 **\r\n")
					cli.PrintAux("**********************************\r\n")

					if opts.keepAlive {
						cli.PrintAux("\r\n\r\n")
						cli.PrintAux("The challenge playground will be kept alive.\r\n")
						cli.PrintAux("Press ENTER to continue...\r\n")
					}

					eventCh <- EventChallengeCompleted
				}

			case EventChallengeCompleted, EventChallengeFailed:
				if chal.Play.IsFailed() {
					// cli.PrintAux("\033c\r") // Reset terminal
					cli.PrintAux("\r\n\r\n")
					cli.PrintAux("************************************************************************\r\n")
					cli.PrintAux("** Oops... 🙈 The challenge playground has been irrecoverably broken. **\r\n")
					cli.PrintAux("************************************************************************\r\n")
				}

				if !opts.keepAlive {
					if sess != nil {
						sess.Close()
						sess = nil
						eventCh <- EventSSHConnEnded
					}

					cli.PrintAux("\r\n\r\nStopping the playground...\r\n")

					if chal, err := cli.Client().StopChallenge(ctx, chal.Name); err != nil {
						cli.PrintErr("Error stopping the challenge: %v\n", err)
					} else if chal.Play == nil || !chal.Play.Active {
						cli.PrintAux("Playground stopped.\r\n")
					}
				}

			case EventWSConnFailed:
				return fmt.Errorf("play connection failed")

			case EventSSHConnEnded:
				cli.PrintAux("\r\n")
				return nil
			}
		}
	}
}

func showSafetyDisclaimerIfNeeded(
	ctx context.Context,
	chalName string,
	cli labcli.CLI,
	consent bool,
) (bool, error) {
	if consent {
		return true, nil
	}

	ch, err := cli.Client().GetChallenge(ctx, chalName)
	if err != nil {
		return false, fmt.Errorf("couldn't get challenge %q: %w", chalName, err)
	}

	if ch.IsOfficial() {
		return true, nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user info: %w", err)
	}

	if ch.IsAuthoredBy(me.ID) {
		return true, nil
	}

	return safety.ShowSafetyDisclaimer(cli)
}
