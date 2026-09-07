package shellgym

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/ide"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/safety"
	issh "github.com/iximiuz/labctl/internal/ssh"
)

const startShellGymTimeout = 10 * time.Minute

type startOptions struct {
	shellGym string
	machine  string
	user     string

	noOpen bool
	noSSH  bool

	skipWaitInit bool

	ide string

	safetyDisclaimerConsent bool

	forwardAgent bool

	asFreeTierUser bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:               "start [flags] <shell-gym-url|shell-gym-name>",
		Short:             `Start a Shell Gym session from the comfort of your local command line`,
		Aliases:           []string{"train"},
		Args:              cobra.MaximumNArgs(1),
		ValidArgsFunction: completion.ShellGymNames(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"shell gym name is required\n\nHint: Use `labctl content list --kind shell-gym` to see your shell gyms",
				)
			}

			opts.shellGym = args[0]
			if strings.HasPrefix(opts.shellGym, "https://") {
				parts := strings.Split(strings.Trim(opts.shellGym, "/"), "/")
				opts.shellGym = parts[len(parts)-1]
			}

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = ide.VSCode
			}

			return labcli.WrapStatusError(runStartShellGym(cmd.Context(), cli, &opts))
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
		`Don't open the shell gym in the browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the shell gym playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.skipWaitInit,
		"skip-wait-init",
		false,
		`Skip waiting for the playground initialization (useful for debugging)`,
	)
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		fmt.Sprintf(`Open the shell gym playground in the IDE by specifying the IDE name (supported: %s)`, ide.SupportedList()),
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

type shellGymEvent string

const (
	EventShellGymReady shellGymEvent = "shell-gym-ready"
	EventSSHConnEnded  shellGymEvent = "ssh-conn-ended"
	EventWSConnFailed  shellGymEvent = "ws-conn-failed"
)

func runStartShellGym(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showSafetyDisclaimerIfNeeded(ctx, opts.shellGym, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

	gym, err := cli.Client().StartShellGym(ctx, opts.shellGym, api.StartShellGymOptions{
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
		AsFreeTierUser:          opts.asFreeTierUser,
	})
	if err != nil {
		return fmt.Errorf("couldn't start the shell gym session: %w", err)
	}

	if gym.Play == nil {
		return fmt.Errorf("shell gym session doesn't have a playground associated with it")
	}

	if opts.machine, err = gym.Play.ResolveMachine(opts.machine); err != nil {
		return err
	}
	if opts.user, err = gym.Play.ResolveUser(opts.machine, opts.user); err != nil {
		return err
	}

	if !opts.noOpen {
		browser.OpenWithFallbackMessage(cli, gym.PageURL)
	}

	playConn := api.NewPlayConn(ctx, gym.Play, cli.Client(), cli.Config().WebSocketOrigin())
	if err := playConn.Start(); err != nil {
		return fmt.Errorf("couldn't start play connection: %w", err)
	}

	eventCh := make(chan shellGymEvent, 100)
	spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	spin.Writer = cli.AuxStream()

	go func() {
		if opts.skipWaitInit {
			cli.PrintAux("WARNING: Not waiting for the playground initialization tasks to complete...\n")
		} else if err := playConn.WaitPlayReady(startShellGymTimeout, spin); err != nil {
			slog.Debug("websocket connection failed", "error", err)

			eventCh <- EventWSConnFailed
			return
		}
		eventCh <- EventShellGymReady
	}()

	if opts.ide != "" {
		go func() {
			cli.PrintAux("Opening local IDE...\n")

			if err := sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
				PlayID:  gym.Play.ID,
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
			case EventShellGymReady:
				if !opts.noSSH {
					cli.PrintAux("SSH-ing into shell gym playground (%s machine)...\n", opts.machine)

					var errCh <-chan error

					sess, errCh, err = ssh.StartSSHSession(ctx, cli, gym.Play, opts.machine, opts.user, nil, opts.forwardAgent)
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
					cli.PrintAux("Shell gym playground is ready! (play ID: %s)\n", gym.Play.ID)
					cli.PrintAux("Shell gym page: %s\n", gym.PageURL)
					return nil
				}

			case EventWSConnFailed:
				return fmt.Errorf("play connection failed")

			case EventSSHConnEnded:
				// The session outlives the SSH connection - the gym's web UI is
				// the main interface, and the schedule only advances on start.
				cli.PrintAux("\r\n")
				cli.PrintAux("The shell gym session is still running: %s\r\n", gym.PageURL)
				cli.PrintAux("Use `labctl shell-gym stop %s` to finish it.\r\n", gym.Name)
				return nil
			}
		}
	}
}

func showSafetyDisclaimerIfNeeded(
	ctx context.Context,
	gymName string,
	cli labcli.CLI,
	consent bool,
) (bool, error) {
	if consent {
		return true, nil
	}

	gym, err := cli.Client().GetShellGym(ctx, gymName)
	if err != nil {
		return false, fmt.Errorf("couldn't get shell gym %q: %w", gymName, err)
	}

	if gym.IsOfficial() {
		return true, nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user info: %w", err)
	}

	if gym.IsAuthoredBy(me.ID) {
		return true, nil
	}

	return safety.ShowSafetyDisclaimer(cli)
}
