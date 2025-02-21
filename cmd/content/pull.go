package content

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type pullOptions struct {
	kind content.ContentKind
	name string

	dirOptions

	force bool
}

func newPullCommand(cli labcli.CLI) *cobra.Command {
	var opts pullOptions

	cmd := &cobra.Command{
		Use:   "pull [flags] <challenge|tutorial|skill-path|course> <name>",
		Short: "Pull remote content files to the local directory for backup or editing",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			return labcli.WrapStatusError(runPullContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	opts.AddDirFlag(flags)

	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Overwrite existing local files without confirmation",
	)

	return cmd
}

func runPullContent(ctx context.Context, cli labcli.CLI, opts *pullOptions) error {
	dir, err := opts.ContentDir(opts.name)
	if err != nil {
		return err
	}

	cont, err := getContent(ctx, cli, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't get content: %w", err)
	}
	cli.PrintAux("Found %s at %s\n", opts.kind, cont.GetPageURL())

	cli.PrintAux("Pulling content files to %s...\n", dir)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("couldn't create directory %s: %w", dir, err)
	}

	files, err := cli.Client().ListContentFiles(ctx, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't list content files: %w", err)
	}

	for _, file := range files {
		cli.PrintAux("Downloading %s\n", file)

		dest := filepath.Join(dir, file)
		if _, err := os.Stat(dest); err == nil {
			cli.PrintAux("File %s already exists.\n", dest)
			if !opts.force && !cli.Confirm("Overwrite?", "Yes", "No") {
				cli.PrintAux("Skipping...\n")
				continue
			}

			cli.PrintAux("Overwriting...\n")
		}

		if err := cli.Client().DownloadContentFile(
			ctx,
			opts.kind,
			opts.name,
			file,
			dest,
		); err != nil {
			return fmt.Errorf("couldn't download content file %s: %w", file, err)
		}
	}

	cli.PrintAux("Done!\n")
	return nil
}
