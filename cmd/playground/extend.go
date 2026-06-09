package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type extendOptions struct {
	playID string

	duration time.Duration

	quiet bool
}

func newExtendCommand(cli labcli.CLI) *cobra.Command {
	var opts extendOptions

	cmd := &cobra.Command{
		Use:               "extend [flags] <playground-id>",
		Short:             `Extend the remaining time of a running playground session`,
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			opts.playID = args[0]

			return labcli.WrapStatusError(runExtendPlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.DurationVarP(
		&opts.duration,
		"duration",
		"d",
		0,
		`Additional time to add (e.g., "2h", "30m", "24h"). Maximum: 24h`,
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

func runExtendPlayground(ctx context.Context, cli labcli.CLI, opts *extendOptions) error {
	if opts.duration <= 0 {
		return labcli.NewStatusError(1, "duration must be positive, e.g., --duration 2h")
	}

	if opts.duration < time.Minute {
		return labcli.NewStatusError(1, "duration must be at least 1 minute")
	}

	if opts.duration > 24*time.Hour {
		return labcli.NewStatusError(1, "duration cannot exceed 24h")
	}

	cli.PrintAux("Extending playground %s by %s...\n", opts.playID, opts.duration)

	play, err := cli.Client().ExtendPlay(ctx, opts.playID, opts.duration)
	if err != nil {
		return fmt.Errorf("couldn't extend the playground: %w", err)
	}

	cli.PrintAux("Playground extended. Expires in %s.\n",
		time.Duration(play.ExpiresIn)*time.Millisecond)

	if !opts.quiet {
		cli.PrintOut("%s\n", play.ID)
	}

	return nil
}
