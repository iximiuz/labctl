package playground

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/safety"
)

const startPlaygroundTimeout = 10 * time.Minute

type startOptions struct {
	playground string
	machine    string
	user       string

	open bool

	ssh bool

	ide bool

	safetyDisclaimerConsent bool

	forwardAgent bool

	skipWaitInit bool

	quiet bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <playground-name>",
		Short: `Start a new playground, possibly open it in a browser`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"playground name is required\n\nAvailable playgrounds:\n%s",
					listKnownPlaygrounds(cmd.Context(), cli))
			}

			if opts.ide && opts.ssh {
				return labcli.NewStatusError(1, "can't use --ide and --ssh flags at the same time")
			}

			opts.playground = args[0]

			return labcli.WrapStatusError(runStartPlayground(cmd.Context(), cli, &opts))
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

	flags.BoolVarP(
		&opts.open,
		"open",
		"o",
		false,
		`Open the playground page in a browser`,
	)
	flags.BoolVar(
		&opts.ssh,
		"ssh",
		false,
		`SSH into the playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.ide,
		"ide",
		false,
		`Open the playground in the IDE (only VSCode is supported at the moment)`,
	)
	flags.BoolVar(
		&opts.safetyDisclaimerConsent,
		"safety-disclaimer-consent",
		false,
		`Acknowledge the safety disclaimer`,
	)
	flags.BoolVar(
		&opts.skipWaitInit,
		"skip-wait-init",
		false,
		`Skip waiting for the playground initialization (useful for debugging)`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print playground's ID`,
	)
	flags.BoolVar(
		&opts.forwardAgent,
		"forward-agent",
		false,
		`INSECURE: Forward the SSH agent to the playground VM (use at your own risk)`,
	)

	return cmd
}

func runStartPlayground(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showSafetyDisclaimerIfNeeded(ctx, opts.playground, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

	play, err := cli.Client().CreatePlay(ctx, api.CreatePlayRequest{
		Playground:              opts.playground,
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
	})
	if err != nil {
		return fmt.Errorf("couldn't create a new playground: %w", err)
	}

	if opts.machine == "" {
		opts.machine = play.Machines[0].Name
	} else {
		if play.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the playground", opts.machine)
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

	cli.PrintAux("New %s playground created with ID %s\n", opts.playground, play.ID)

	if opts.open {
		cli.PrintAux("Opening %s in your browser...\n", play.PageURL)

		if err := open.Run(play.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	if opts.ide {
		return sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
			PlayID:  play.ID,
			Machine: opts.machine,
			User:    opts.user,
			IDE:     true,
		})
	}

	if opts.skipWaitInit {
		cli.PrintAux("WARNING: Not waiting for the playground initialization tasks to complete...\n")
	} else if len(play.Tasks) > 0 {
		playConn := api.NewPlayConn(ctx, play, cli.Client(), cli.Config().WebSocketOrigin())
		if err := playConn.Start(); err != nil {
			return fmt.Errorf("couldn't start play connection: %w", err)
		}

		spin := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
		spin.Writer = cli.AuxStream()
		if err := playConn.WaitPlayReady(startPlaygroundTimeout, spin); err != nil {
			return fmt.Errorf("playground initialization failed: %w", err)
		}
	}

	if opts.ssh {
		cli.PrintAux("SSH-ing into %s machine...\n", opts.machine)

		if sess, err := ssh.StartSSHSession(ctx, cli, play.ID, opts.machine, opts.user, nil, opts.forwardAgent); err != nil {
			return fmt.Errorf("couldn't start SSH session: %w", err)
		} else {
			if err := sess.Wait(); err != nil {
				slog.Debug("SSH session wait said: " + err.Error())
			}
			return nil
		}
	}

	cli.PrintOut("%s\n", play.ID)

	return nil
}

func listKnownPlaygrounds(ctx context.Context, cli labcli.CLI) string {
	playgrounds, err := cli.Client().ListPlaygrounds(ctx, nil)
	if err != nil {
		cli.PrintErr("Couldn't list known playgrounds: %v\n", err)
		return ""
	}

	var res []string
	for _, p := range playgrounds {
		res = append(res, fmt.Sprintf("  - %s - %s", p.Name, p.Description))
	}

	return strings.Join(res, "\n")
}

func showSafetyDisclaimerIfNeeded(ctx context.Context, playgroundName string, cli labcli.CLI, consent bool) (bool, error) {
	if consent {
		return true, nil
	}

	playground, err := cli.Client().GetPlayground(ctx, playgroundName, nil)
	if err != nil {
		return false, fmt.Errorf("couldn't get the playground: %w", err)
	}

	if playground.Owner == "" { // official playgrounds don't need consent
		return true, nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return false, fmt.Errorf("couldn't get the current user info: %w", err)
	}

	if me.ID == playground.Owner {
		return true, nil
	}

	return safety.ShowSafetyDisclaimer(cli)
}
