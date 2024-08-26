package challenge

import (
	"context"
	"fmt"
	"strings"

	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/internal/labcli"
)

type startOptions struct {
	challenge string
	machine   string
	user      string

	noOpen bool

	noSSH bool

	ide bool
}

func newStartCommand(cli labcli.CLI) *cobra.Command {
	var opts startOptions

	cmd := &cobra.Command{
		Use:   "start [flags] <challenge-name>",
		Short: `Start solving a challenge from the catalog`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.NewStatusError(1,
					"challenge name is required\n\nAvailable challenges:\n%s",
					listKnownChallenges(cmd.Context(), cli))
			}

			if opts.ide {
				opts.noSSH = true
			}

			opts.challenge = args[0]

			return labcli.WrapStatusError(runStartChallenge(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.machine,
		"machine",
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
		`Don't open the challenge page in a browser`,
	)
	flags.BoolVar(
		&opts.noSSH,
		"no-ssh",
		false,
		`Don't SSH into the challenge playground immediately after it's created`,
	)
	flags.BoolVar(
		&opts.ide,
		"ide",
		false,
		`Open the challenge playground in the IDE (only VSCode is supported at the moment)`,
	)

	return cmd
}

func runStartChallenge(ctx context.Context, cli labcli.CLI, opts *startOptions) error {
	chal, err := cli.Client().StartChallenge(ctx, opts.challenge)
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

	if opts.ide {
		return sshproxy.RunSSHProxy(ctx, cli, &sshproxy.Options{
			PlayID:  chal.Play.ID,
			Machine: opts.machine,
			User:    opts.user,
			IDE:     true,
		})
	}

	if !opts.noSSH {
		cli.PrintAux("SSH-ing into challenge playground (%s machine)...\n", opts.machine)

		return ssh.RunSSHSession(ctx, cli, chal.Play.ID, opts.machine, opts.user, nil)
	}

	return nil
}

func listKnownChallenges(ctx context.Context, cli labcli.CLI) string {
	challenges, err := cli.Client().ListChallenges(ctx)
	if err != nil {
		cli.PrintErr("Couldn't list challenges: %v\n", err)
		return ""
	}

	var res []string
	for _, ch := range challenges {
		res = append(res, fmt.Sprintf("[%s] %s - %s %s",
			strings.Join(ch.Categories, ", "),
			ch.Name,
			ch.Description,
			"#"+strings.Join(ch.Tags, ", #"),
		))
	}

	return strings.Join(res, "\n")
}
