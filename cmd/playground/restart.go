package playground

import (
	"context"
	"errors"
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
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
)

const restartCommandTimeout = 5 * time.Minute

type restartOptions struct {
	playId  string
	machine string
	user    string

	open bool
	ide  string
	ssh  bool

	forwardAgent bool

	withPortForwards bool

	quiet bool
}

var (
	// can be a playID or a title of the playground
	playgroundIdentifier string
)

func newRestartCommand(cli labcli.CLI) *cobra.Command {
	var opts restartOptions

	cmd := &cobra.Command{
		Use:               "restart [flags] <playground-id|title>",
		Short:             `Restart a stopped playground session, resuming its state`,
		ValidArgsFunction: completion.StoppedPlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if cmd.Flags().Changed("ide") && opts.ide == "" {
				opts.ide = sshproxy.IDEVSCode
			}

			if opts.ide != "" && opts.ssh {
				return labcli.NewStatusError(1, "can't use --ide and --ssh flags at the same time")
			}

			playgroundIdentifier = args[0]
			return labcli.WrapStatusError(runRestartPlayground(cmd.Context(), cli, &opts, playgroundIdentifier))
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
	flags.BoolVar(
		&opts.withPortForwards,
		"with-port-forwards",
		false,
		`Automatically forward ports specified in the playground's config or forwarded during the previous run(s)`,
	)

	return cmd
}

func restartWithPlaygroundId(ctx context.Context, cli labcli.CLI, opts *restartOptions) error {
	play, err := cli.Client().RestartPlay(ctx, opts.playId)
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
		if play, err := cli.Client().GetPlay(ctx, opts.playId); err == nil {
			if play.StateIs(api.StateRunning) {
				s.FinalMSG = "Waiting for playground to restart... Done.\n"
				s.Stop()
				break
			}

			s.Prefix = fmt.Sprintf("Waiting for playground to restart... (%s) ", play.State())
		}

		time.Sleep(2 * time.Second)
	}

	cli.PrintAux("Playground has been restarted.\n")

	if opts.machine, err = play.ResolveMachine(opts.machine); err != nil {
		return err
	}
	if opts.user, err = play.ResolveUser(opts.machine, opts.user); err != nil {
		return err
	}

	if opts.open {
		browser.OpenWithFallbackMessage(cli, play.PageURL)
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

	// If only --port-forward was provided (no --ide or --ssh), wait for it
	if portForwardErrCh != nil {
		return <-portForwardErrCh
	}

	return nil
}

func isItPlaygroundIDCheck(id string) bool {
	if len(id) != 24 {
		return false
	}
	// check if each charachter within the string falls in the hex range
	for _, c := range id {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return false
		}
	}
	return true
}

func runRestartPlayground(ctx context.Context, cli labcli.CLI, opts *restartOptions, playgroundIdentifier string) error {
	cli.PrintAux("Restarting playground %s...\n", playgroundIdentifier)

	// if the provided identifier is a confirmed playgroundID restart it immediately
	if isItPlaygroundIDCheck(playgroundIdentifier) {
		opts.playId = playgroundIdentifier
		return restartWithPlaygroundId(ctx, cli, opts)
	}

	// if identifier is not a playgroundID search for playgrounds with matching titles
	// and if found one restart it.
	var matchingPlaygrounds []api.Play

	cli.PrintAux("Searching for a playground with title: %s\n", playgroundIdentifier)
	playgroundsList, err := cli.Client().ListPlays(ctx, api.ListPlaysQueryParams{Persistent: true})
	if err != nil {
		return fmt.Errorf("couldn't get a list of playgrounds: %w", err)
	}
	for _, pgItem := range playgroundsList {
		if pgItem.Title == playgroundIdentifier {
			matchingPlaygrounds = []api.Play{*pgItem}
			break
		}

		if strings.HasPrefix(pgItem.Title, playgroundIdentifier) {
			matchingPlaygrounds = append(matchingPlaygrounds, *pgItem)
		}
	}

	if len(matchingPlaygrounds) == 0 {
		return errors.New("unable to find a playground with provided title")
	}

	if len(matchingPlaygrounds) > 1 {
		return errors.New("unambigious title, please use the full title of a playground or a longer prefix")
	}

	opts.playId = matchingPlaygrounds[0].ID
	return restartWithPlaygroundId(ctx, cli, opts)
}
