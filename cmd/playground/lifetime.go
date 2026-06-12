package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type lifetimeOptions struct {
	playID   string
	lifetime time.Duration
}

func newLifetimeCommand(cli labcli.CLI) *cobra.Command {
	var opts lifetimeOptions

	cmd := &cobra.Command{
		Use:               "lifetime <playground-id> [new-lifetime]",
		Short:             `Show or set a playground's lifetime - the total session duration counted from the playground's start (e.g., 90m, 3h, 2h30m)`,
		Args:              cobra.RangeArgs(1, 2),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]

			if len(args) == 1 {
				return labcli.WrapStatusError(runShowPlaygroundLifetime(cmd.Context(), cli, &opts))
			}

			lifetime, err := time.ParseDuration(args[1])
			if err != nil {
				return labcli.WrapStatusError(fmt.Errorf("invalid lifetime %q: %w", args[1], err))
			}
			opts.lifetime = lifetime

			return labcli.WrapStatusError(runSetPlaygroundLifetime(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runShowPlaygroundLifetime(ctx context.Context, cli labcli.CLI, opts *lifetimeOptions) error {
	play, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if play.MaxPlayTime == "" {
		return labcli.NewStatusError(1, "playground %s has no lifetime information", opts.playID)
	}

	lifetime, err := time.ParseDuration(play.MaxPlayTime)
	if err != nil {
		cli.PrintOut("%s\n", play.MaxPlayTime)
		return nil
	}

	if play.ExpiresIn > 0 {
		cli.PrintOut("%s (expires in %s)\n", lifetime, (time.Duration(play.ExpiresIn) * time.Millisecond).Round(time.Second))
	} else {
		cli.PrintOut("%s\n (expired)", lifetime)
	}

	return nil
}

func runSetPlaygroundLifetime(ctx context.Context, cli labcli.CLI, opts *lifetimeOptions) error {
	minutes := int(opts.lifetime.Minutes())
	if minutes < 1 {
		return fmt.Errorf("lifetime must be at least 1 minute")
	}

	cli.PrintAux("Setting lifetime of playground %s to %s...\n", opts.playID, opts.lifetime)

	play, err := cli.Client().SetPlayMaxPlayTime(ctx, opts.playID, minutes)
	if err != nil {
		return fmt.Errorf("couldn't set the playground lifetime: %w", err)
	}

	if play.ExpiresIn > 0 {
		cli.PrintAux("Playground will now expire in %s.\n", (time.Duration(play.ExpiresIn) * time.Millisecond).Round(time.Second))
	} else {
		cli.PrintAux("Playground lifetime is set.\n")
	}

	return nil
}
