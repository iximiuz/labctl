package content

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type pushOptions struct {
	kind content.ContentKind
	name string

	dirOptions

	force bool
}

func newPushCommand(cli labcli.CLI) *cobra.Command {
	var opts pushOptions

	cmd := &cobra.Command{
		Use:   "push [flags] <challenge|tutorial|course> <name>",
		Short: `Push content files from the local directory to the remote content repository (the "inner author loop").`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			return labcli.WrapStatusError(runPushContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	opts.AddDirFlag(flags)

	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Overwrite existing remote files with the local ones and delete remote files that don't exist locally",
	)

	return cmd
}

func runPushContent(ctx context.Context, cli labcli.CLI, opts *pushOptions) error {
	dir, err := opts.ContentDir(opts.name)
	if err != nil {
		return err
	}

	if _, err := os.Stat(dir); err != nil {
		cli.PrintAux("Directory %s doesn't exist. You may need to `labctl content pull %s %s --dir %s` first.\n", dir)
		return errors.New("push failed")
	}

	remoteFiles, err := cli.Client().ListContentFiles(ctx, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't list remote content files: %w", err)
	}

	localFiles, err := listContentFilesLocal(dir)
	if err != nil {
		return fmt.Errorf("couldn't list local content files: %w", err)
	}

	var merr error

	// Upload new and update existing files.
	for _, abspath := range localFiles {
		relpath := strings.TrimPrefix(strings.TrimPrefix(abspath, dir), string(filepath.Separator))

		cli.PrintAux("Pushing %s\n", relpath)

		// TODO: If file exists remotely, ask permission to overwrite
		if slices.Contains(remoteFiles, relpath) && !opts.force {
			if !cli.Confirm(fmt.Sprintf("File %s already exists remotely. Overwrite?", relpath), "Yes", "No") {
				cli.PrintAux("Skipping...\n")
				continue
			}
		}

		cli.PrintAux("Uploading...\n")

		if filepath.Ext(relpath) == ".md" {
			content, err := os.ReadFile(abspath)
			if err != nil {
				merr = errors.Join(merr, fmt.Errorf("couldn't read content markdown %s: %w", relpath, err))
				continue
			}

			if err := cli.Client().PutContentMarkdown(
				ctx,
				opts.kind,
				opts.name,
				relpath,
				string(content),
			); err != nil {
				merr = errors.Join(merr, fmt.Errorf("couldn't upload content markdown %s: %w", relpath, err))
			}
		} else {
			if err := cli.Client().UploadContentFile(
				ctx,
				opts.kind,
				opts.name,
				relpath,
				abspath,
			); err != nil {
				merr = errors.Join(merr, fmt.Errorf("couldn't upload content file %s: %w", relpath, err))
			}
		}
	}

	// Delete remote files that don't exist locally.
	for _, relpath := range remoteFiles {
		if slices.Contains(localFiles, filepath.Join(dir, relpath)) {
			continue
		}

		if !opts.force && !cli.Confirm(fmt.Sprintf("File %s doesn't exist locally. Delete remotely?", relpath), "Yes", "No") {
			cli.PrintAux("Skipping...\n")
			continue
		}

		cli.PrintAux("Deleting remote %s\n", relpath)

		if err := cli.Client().DeleteContentFile(ctx, opts.kind, opts.name, relpath); err != nil {
			merr = errors.Join(merr, fmt.Errorf("couldn't delete remote content file %s: %w", relpath, err))
		}
	}

	return merr
}

func listContentFilesLocal(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("couldn't list directory: %w", err)
	}

	var result []string
	for _, file := range files {
		if file.IsDir() {
			files, err := listContentFilesLocal(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}
			result = append(result, files...)
		} else {
			result = append(result, filepath.Join(dir, file.Name()))
		}
	}

	return result, nil
}
