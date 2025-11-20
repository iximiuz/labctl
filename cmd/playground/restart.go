package playground

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/labcli"
)

const restartCommandTimeout = 5 * time.Minute

type restartOptions struct {
	playID string

	machine string
	user    string

	open bool
	ide  string
	ssh  bool

	forwardAgent bool

	quiet bool
}

func newRestartCommand(cli labcli.CLI) *cobra.Command {
	var opts restartOptions

	cmd := &cobra.Command{
		Use:   "restart [flags] <playground-id>",
		Short: `Restart a stopped playground session, resuming its state`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

			if opts.ide != "" && opts.ssh {
				return labcli.NewStatusError(1, "can't use --ide and --ssh flags at the same time")
			}

			opts.playID = args[0]

			return labcli.WrapStatusError(runRestartPlayground(cmd.Context(), cli, &opts))
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
	flags.StringVar(
		&opts.ide,
		"ide",
		"",
		`Open the playground in the IDE by specifying the IDE name (supported: "code", "cursor", "windsurf")`,
	)
	flags.BoolVar(
		&opts.ssh,
		"ssh",
		false,
		`SSH into the playground immediately after it's started`,
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
		`Do not print any diagnostic messages`,
	)

	return cmd
}

func runRestartPlayground(ctx context.Context, cli labcli.CLI, opts *restartOptions) error {
	cli.PrintAux("Restarting playground %s...\n", opts.playID)

	play, err := cli.Client().RestartPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't restart the playground: %w", err)
	}

	s := spinner.New(spinner.CharSets[38], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for playground to restart... "
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, restartCommandTimeout)
	defer cancel()

	for ctx.Err() == nil {
		if play, err := cli.Client().GetPlay(ctx, opts.playID); err == nil && !play.IsActive() {
			s.FinalMSG = "Waiting for playground to restart... Done.\n"
			s.Stop()

			return nil
		}

		time.Sleep(2 * time.Second)
	}

	cli.PrintAux("Playground has been restarted.\n")

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

	if opts.ssh {
		cli.PrintAux("SSH-ing into %s machine...\n", opts.machine)

		sess, errCh, err := ssh.StartSSHSession(ctx, cli, play.ID, opts.machine, opts.user, nil, opts.forwardAgent)
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
	return nil
}
