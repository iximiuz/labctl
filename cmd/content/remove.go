package content

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

type removeOptions struct {
	kind  ContentKind
	name  string
	force bool
}

func newRemoveCommand(cli labcli.CLI) *cobra.Command {
	var opts removeOptions

	cmd := &cobra.Command{
		Use:   "remove [flags] <challenge|tutorial|course> <name>",
		Short: "Remove a piece of content you authored.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			return labcli.WrapStatusError(runRemoveContent(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runRemoveContent(ctx context.Context, cli labcli.CLI, opts *removeOptions) error {
	cli.PrintAux("Removing %s %s...\n", opts.kind, opts.name)

	if !opts.force {
		if !cli.Confirm(
			"This action is irreversible. Are you sure?",
			"Yes", "No",
		) {
			return labcli.NewStatusError(0, "Glad you changed your mind!")
		}
	}

	switch opts.kind {
	case KindChallenge:
		return cli.Client().DeleteChallenge(ctx, opts.name)

	case KindTutorial:
		return cli.Client().DeleteTutorial(ctx, opts.name)

	case KindCourse:
		return fmt.Errorf("removing courses is not supported yet")

	default:
		return fmt.Errorf("unknown content kind %q", opts.kind)
	}
}
