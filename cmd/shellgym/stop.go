package shellgym

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type stopOptions struct {
	shellGym string

	quiet bool
}

func newStopCommand(cli labcli.CLI) *cobra.Command {
	var opts stopOptions

	cmd := &cobra.Command{
		Use:               "stop [flags] <shell-gym-url|shell-gym-name>",
		Short:             `Finish the running session of a shell gym`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ShellGymNames(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.shellGym = args[0]
			if strings.HasPrefix(opts.shellGym, "https://") {
				parts := strings.Split(strings.Trim(opts.shellGym, "/"), "/")
				opts.shellGym = parts[len(parts)-1]
			}

			return labcli.WrapStatusError(runStopShellGym(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Do not print any diagnostic messages`,
	)

	return cmd
}

func runStopShellGym(ctx context.Context, cli labcli.CLI, opts *stopOptions) error {
	cli.PrintAux("Finishing the session for shell gym %s...\n", opts.shellGym)

	gym, err := cli.Client().GetShellGym(ctx, opts.shellGym)
	if err != nil {
		return fmt.Errorf("couldn't get the shell gym: %w", err)
	}

	if gym.Play == nil || !gym.Play.IsActive() {
		cli.PrintErr("Shell gym has no running session - nothing to stop.\n")
		return nil
	}

	if _, err = cli.Client().StopShellGym(ctx, opts.shellGym); err != nil {
		return fmt.Errorf("couldn't stop the shell gym session: %w", err)
	}

	cli.PrintAux("Shell gym session has been finished.\n")
	return nil
}
