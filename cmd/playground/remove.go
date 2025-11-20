package playground

import (
	"context"
	"fmt"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/spf13/cobra"
)

type removeOptions struct {
	name  string
	force bool
}

func newRemoveCommand(cli labcli.CLI) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "remove [flags] <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a playground you authored",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]

			return labcli.WrapStatusError(runRemovePlayground(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Remove without confirmation",
	)

	return cmd
}

func runRemovePlayground(ctx context.Context, cli labcli.CLI, opts *removeOptions) error {
	if !opts.force {
		cli.PrintAux("Removing %s ...\n", opts.name)

		_, err := cli.Client().GetPlayground(ctx, opts.name, nil)
		if err != nil {
			return fmt.Errorf("playground doesn't exist: %s", opts.name)
		}

		if !cli.Confirm(
			"This action is irreversible. Are you sure?",
			"Yes", "No",
		) {
			return labcli.NewStatusError(0, "Glad you changed your mind!")
		}
	}

	return cli.Client().DeletePlayground(ctx, opts.name)
}
