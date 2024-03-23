package content

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
	fmt.Println(remoteFiles)

	localFiles, err := listContentFilesLocal(dir)
	if err != nil {
		return fmt.Errorf("couldn't list local content files: %w", err)
	}

	for _, file := range localFiles {
		cli.PrintAux("Uploading %s\n", file)

		// TODO: If file exists remotely, ask permission to overwrite

		if err := cli.Client().UploadContentFile(
			ctx,
			opts.kind,
			opts.name,
			strings.TrimPrefix(strings.TrimPrefix(file, dir), string(filepath.Separator)),
			file,
		); err != nil {
			return fmt.Errorf("couldn't upload content file %s: %w", file, err)
		}
	}

	// TODO: ...
	// for _, file := range remoteFiles {
	// 	if !contains(localFiles, file) {
	// 		cli.PrintAux("Deleting %s\n", file)

	// 		if err := cli.Client().DeleteContentFile(ctx, opts.kind, opts.name, file); err != nil {
	// 			return fmt.Errorf("couldn't delete content file %s: %w", file, err)
	// 		}
	// 	}
	// }

	// TODO: --stream
	// cli.PrintAux("Starting content sync in %s...\n", opts.dir)
	//
	// for ctx.Err() == nil {
	// 	data, err := os.ReadFile(filepath.Join(opts.dir, "index.md"))
	// 	if err != nil {
	// 		return fmt.Errorf("couldn't read index.md: %w", err)
	// 	}

	// 	if err := cli.Client().PutContentMarkdown(ctx, api.PutContentMarkdownRequest{
	// 		Kind:    opts.kind.String(),
	// 		Name:    opts.name,
	// 		Content: string(data),
	// 	}); err != nil {
	// 		return fmt.Errorf("couldn't update content: %w", err)
	// 	}

	// 	cli.PrintAux("Synced content...\n")
	// 	time.Sleep(5 * time.Second)
	// }

	return nil
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
