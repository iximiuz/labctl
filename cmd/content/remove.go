package content

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type removeOptions struct {
	kind content.ContentKind
	name string

	force bool
}

func newRemoveCommand(cli labcli.CLI) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:     "remove [flags] <challenge|tutorial|course> <name>",
		Aliases: []string{"rm"},
		Short:   "Remove a piece of content you authored.",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			return labcli.WrapStatusError(runRemoveContent(cmd.Context(), cli, &opts))
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

func runRemoveContent(ctx context.Context, cli labcli.CLI, opts *removeOptions) error {
	if !opts.force {
		cli.PrintAux("Removing %s %s...\n", opts.kind, opts.name)

		if !cli.Confirm(
			"This action is irreversible. Are you sure?",
			"Yes", "No",
		) {
			return labcli.NewStatusError(0, "Glad you changed your mind!")
		}
	}

	switch opts.kind {
	case content.KindChallenge:
		return cli.Client().DeleteChallenge(ctx, opts.name)

	case content.KindTutorial:
		return cli.Client().DeleteTutorial(ctx, opts.name)

	case content.KindCourse:
		return cli.Client().DeleteCourse(ctx, opts.name)

	default:
		return fmt.Errorf("unknown content kind %q", opts.kind)
	}
}
