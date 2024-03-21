package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type syncOptions struct {
	dir string
}

func newSyncCommand(cli labcli.CLI) *cobra.Command {
	var opts syncOptions

	cmd := &cobra.Command{
		Use:   "sync [flags] [content-dir]",
		Short: "Sync a local directory to remote content storage - the main content authoring routine.",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				opts.dir = args[0]
			} else {
				if cwd, err := os.Getwd(); err != nil {
					return labcli.WrapStatusError(fmt.Errorf("couldn't get the current working directory: %w", err))
				} else {
					opts.dir = cwd
				}
			}

			return labcli.WrapStatusError(runSyncContent(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runSyncContent(ctx context.Context, cli labcli.CLI, opts *syncOptions) error {
	cli.PrintAux("Starting content sync in %s...\n", opts.dir)

	for ctx.Err() == nil {
		data, err := os.ReadFile(filepath.Join(opts.dir, "index.md"))
		if err != nil {
			return fmt.Errorf("couldn't read index.md: %w", err)
		}

		if err := cli.Client().PutContentMarkdown(ctx, api.PutContentMarkdownRequest{
			Kind:    "challenge",
			Name:    "foobar-qux",
			Content: string(data),
		}); err != nil {
			return fmt.Errorf("couldn't update content: %w", err)
		}

		cli.PrintAux("Synced content...\n")
		time.Sleep(5 * time.Second)
	}

	return nil
}
