package playground

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

const persistCommandTimeout = 5 * time.Minute

type persistOptions struct {
	playID string
}

func newPersistCommand(cli labcli.CLI) *cobra.Command {
	var opts persistOptions

	cmd := &cobra.Command{
		Use:   "persist [flags] <playground-id>",
		Short: `Makes an active playground session persistent`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]

			return labcli.WrapStatusError(runPersistPlayground(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runPersistPlayground(ctx context.Context, cli labcli.CLI, opts *persistOptions) error {
	cli.PrintAux("Persisting playground %s...\n", opts.playID)

	if err := cli.Client().PersistPlay(ctx, opts.playID); err != nil {
		return fmt.Errorf("couldn't persist the playground: %w", err)
	}

	cli.PrintAux("Playground is persistent.\n")

	return nil
}
