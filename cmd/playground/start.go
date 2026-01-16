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

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/safety"
)

const startPlaygroundTimeout = 10 * time.Minute

type startOptions struct {
	playground string
	machine    string
	user       string

	file string
	open bool
	ssh  bool
	ide  string

	forwardAgent     bool
	skipWaitInit     bool
	withPortForwards bool

	safetyDisclaimerConsent bool

	asFreeTierUser bool

	initConditions map[string]string

	quiet bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <playground-name>",
		Short: `Start a new playground session, possibly opening it in a browser`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"playground name is required\n\nAvailable playgrounds:\n%s",
					listKnownPlaygrounds(cmd.Context(), cli))
			}

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

			if opts.ide != "" && opts.ssh {
				return labcli.NewStatusError(1, "can't use --ide and --ssh flags at the same time")
			}

			opts.playground = args[0]

			return labcli.WrapStatusError(runStartPlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.file,
		"file",
		"f",
		"",
		`Path to a manifest file with playground configuration (machines, tabs, custom init tasks, etc.)`,
	)
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
		`SSH into the playground immediately after it's started`,
	)
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		`Open the playground in the IDE by specifying the IDE name (supported: "code", "cursor", "windsurf")`,
	)
	flags.BoolVar(
		&opts.asFreeTierUser,
		"as-free-tier-user",
		false,
		`Run this playground as a free tier user (handy for testing that the playground works on all tiers)`,
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
	flags.StringToStringVarP(
		&opts.initConditions,
		"init-condition",
		"i",
		nil,
		`Set init conditions as key-value pairs (can be used multiple times)`,
	)
	flags.BoolVar(
		&opts.withPortForwards,
		"with-port-forwards",
		false,
		`Automatically forward ports specified in the playground's config`,
	)

	return cmd
}

func runStartPlayground(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	var err error
	opts.safetyDisclaimerConsent, err = showSafetyDisclaimerIfNeeded(ctx, opts.playground, cli, opts.safetyDisclaimerConsent)
	if err != nil {
		return err
	}

	// Parse manifest file if provided
	var manifest *api.PlaygroundManifest
	if opts.file != "" {
		manifest, err = readManifestFile(opts.file)
		if err != nil {
			return fmt.Errorf("couldn't read manifest file: %w", err)
		}
	}

	// Build CreatePlay request
	req := api.CreatePlayRequest{
		Playground:              opts.playground,
		InitConditions:          opts.initConditions,
		SafetyDisclaimerConsent: opts.safetyDisclaimerConsent,
		AsFreeTierUser:          opts.asFreeTierUser,
	}

	// Add manifest fields if available
	if manifest != nil {
		req.Tabs = manifest.Playground.Tabs
		req.Networks = manifest.Playground.Networks
		req.Machines = manifest.Playground.Machines
		if len(manifest.Playground.InitTasks) > 0 {
			req.InitTasks = manifest.Playground.InitTasks
		}
	}

	play, err := cli.Client().CreatePlay(ctx, req)
	if err != nil {
		return fmt.Errorf("couldn't start the playground: %w", err)
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

	cli.PrintAux("New %s playground started with ID %s\n", opts.playground, play.ID)

	if opts.open {
		cli.PrintAux("Opening %s in your browser...\n", play.PageURL)

		if err := open.Run(play.PageURL); err != nil {
			cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually to access the playground.\n")
		}
	}

	if opts.ide != "" {
		return sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
			PlayID:  play.ID,
			Machine: opts.machine,
			User:    opts.user,
			IDE:     opts.ide,
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

	// Start port forwarding if requested.
	// If combined with --ide or --ssh, run in background; otherwise block.
	var portForwardErrCh <-chan error
	if opts.withPortForwards {
		var err error
		portForwardErrCh, err = portforward.RestoreSavedForwards(ctx, cli.Client(), play.ID, cli)
		if err != nil {
			return err
		}
	}

	if opts.ssh {
		cli.PrintAux("SSH-ing into %s machine...\n", opts.machine)

		sess, errCh, err := ssh.StartSSHSession(ctx, cli, play, opts.machine, opts.user, nil, opts.forwardAgent)
		if err != nil {
			return fmt.Errorf("couldn't start SSH session: %w", err)
		}

		if err := <-errCh; err != nil {
			return fmt.Errorf("SSH session error: %w", err)
		}

		if err := sess.Wait(); err != nil {
			slog.Debug("SSH session wait said: " + err.Error())
		}

		return nil
	}

	cli.PrintOut("%s\n", play.ID)

	// If only --with-port-forwards was provided (no --ide or --ssh), wait for it
	if portForwardErrCh != nil {
		return <-portForwardErrCh
	}

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
