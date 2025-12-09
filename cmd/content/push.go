package content

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/sourcegraph/conc/pool"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type pushOptions struct {
	kind content.ContentKind
	name string

	DirOptions

	watch bool

	force bool
}

func (opts *pushOptions) toConfig() (PushConfig, error) {
	config := PushConfig{
		Kind:  opts.kind,
		Name:  opts.name,
		Force: opts.force,
	}

	dir, err := opts.ContentDir(opts.name)
	if err != nil {
		return config, err
	}

	if _, err := os.Stat(dir); err != nil {
		return config, fmt.Errorf("directory %s doesn't exist or is not accessible", dir)
	}

	config.Dir = dir

	return config, nil
}

func newPushCommand(cli labcli.CLI) *cobra.Command {
	var opts pushOptions

	cmd := &cobra.Command{
		Use:   "push [flags] <challenge|tutorial|skill-path|course|training> <name>",
		Short: `Push content files from the local directory to the remote content repository (the "inner author loop").`,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := opts.kind.Set(args[0]); err != nil {
				return labcli.WrapStatusError(err)
			}
			opts.name = args[1]

			if opts.watch && !opts.force {
				return labcli.WrapStatusError(errors.New("watch mode requires --force flag"))
			}

			return labcli.WrapStatusError(runPushContent(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	opts.AddDirFlag(flags, "")

	flags.BoolVarP(
		&opts.watch,
		"watch",
		"w",
		false,
		"Watch the local directory for changes and push them automatically (inner author loop)",
	)
	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Overwrite existing remote files with the local ones and delete remote files that don't exist locally without confirmation",
	)

	return cmd
}

func runPushContent(ctx context.Context, cli labcli.CLI, opts *pushOptions) error {
	config, err := opts.toConfig()
	if err != nil {
		return fmt.Errorf("push failed: %w", err)
	}

	cont, err := getContent(ctx, cli, opts.kind, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't get content: %w", err)
	}
	cli.PrintAux("Found %s at %s\n", opts.kind, cont.GetPageURL())

	if opts.watch {
		return RunPushWatch(ctx, cli, config)
	} else {
		return RunPushOnce(ctx, cli, config)
	}
}

type PushConfig struct {
	Kind  content.ContentKind
	Name  string
	Dir   string
	Force bool
}

type pushState struct {
	dir string

	remoteFiles map[string]string

	localFiles map[string]string
}

func (s *pushState) toUpload() []string {
	var files []string
	for file, digest := range s.localFiles {
		if s.remoteFiles[file] == "" || s.remoteFiles[file] != digest {
			files = append(files, file)
		}
	}

	return files
}

func (s *pushState) toDelete() []string {
	var files []string
	for file := range s.remoteFiles {
		if _, ok := s.localFiles[file]; !ok {
			files = append(files, file)
		}
	}

	return files
}

func RunPushOnce(ctx context.Context, cli labcli.CLI, config PushConfig) error {
	var (
		state pushState = pushState{dir: config.Dir}
		err   error
	)

	state.remoteFiles, err = listContentFilesRemote(ctx, cli.Client(), config.Kind, config.Name)
	if err != nil {
		return fmt.Errorf("couldn't list remote content files: %w", err)
	}

	state.localFiles, err = listContentFilesLocal(config.Dir)
	if err != nil {
		return fmt.Errorf("couldn't list local content files: %w", err)
	}

	return reconcileContentState(ctx, cli, config, state)
}

func RunPushWatch(ctx context.Context, cli labcli.CLI, config PushConfig) error {
	var (
		state pushState = pushState{dir: config.Dir}
		err   error
	)

	state.remoteFiles, err = listContentFilesRemote(ctx, cli.Client(), config.Kind, config.Name)
	if err != nil {
		return fmt.Errorf("couldn't list remote content files: %w", err)
	}

	state.localFiles, err = listContentFilesLocal(config.Dir)
	if err != nil {
		return fmt.Errorf("couldn't list local content files: %w", err)
	}

	// Initial push.
	if err := reconcileContentState(ctx, cli, config, state); err != nil {
		cli.PrintErr("\n⚠️ WARNING: %s\n\n", err)
	}

	// Watch for changes.
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("couldn't create watcher: %w", err)
	}
	defer watcher.Close()

	if err := addWatchDirs(cli, watcher, state); err != nil {
		return err
	}

	for ctx.Err() == nil {
		select {
		case <-ctx.Done():
			return nil

		case <-watcher.Events:
			state.localFiles, err = listContentFilesLocal(config.Dir)
			if err != nil {
				return fmt.Errorf("couldn't list local content files: %w", err)
			}

			if err := reconcileContentState(ctx, cli, config, state); err != nil {
				// Don't break the loop on error - if it's an HTTP 400 because of malformed content,
				// it will go away by itself when the user fixes the content.
				// If it's an HTTP 5xx, it'll go away when the server is fixed.
				cli.PrintErr("\n⚠️ WARNING: %s\n\n", err)
				time.Sleep(1 * time.Second)
			}

			if err := addWatchDirs(cli, watcher, state); err != nil {
				return err
			}

		case err := <-watcher.Errors:
			return fmt.Errorf("watcher error: %w", err)
		}
	}

	return ctx.Err()
}

func reconcileContentState(ctx context.Context, cli labcli.CLI, config PushConfig, state pushState) error {
	// Arbitrary max goroutines to not overload the server
	const concurrency = 3

	p := pool.New().WithMaxGoroutines(concurrency).WithErrors().WithContext(ctx)

	var mu sync.Mutex

	// Upload new and update existing files.
	for _, file := range state.toUpload() {
		// Skip reconciling index.md and manifest files for playgrounds
		if isExcluded(config.Kind, file) {
			continue
		}

		cli.PrintAux("🌍 Pushing %s\n", file)

		if _, found := state.remoteFiles[file]; found && !config.Force {
			if !cli.Confirm(fmt.Sprintf("File %s already exists remotely. Overwrite?", file), "Yes", "No") {
				cli.PrintAux("Skipping...\n")
				continue
			}
		}

		p.Go(func(ctx context.Context) error {
			if filepath.Ext(file) == ".md" {
				content, err := os.ReadFile(filepath.Join(state.dir, file))
				if err != nil {
					return fmt.Errorf("couldn't read content markdown file %q: %w", file, err)
				}

				if err := cli.Client().PutContentMarkdown(
					ctx,
					config.Kind,
					config.Name,
					file,
					string(content),
				); err != nil {
					return fmt.Errorf("couldn't upload content markdown file %q: %w", file, err)
				}
			} else {
				if err := cli.Client().UploadContentFile(
					ctx,
					config.Kind,
					config.Name,
					file,
					filepath.Join(state.dir, file),
				); err != nil {
					return fmt.Errorf("couldn't upload content file %q: %w", file, err)
				}
			}

			mu.Lock()
			state.remoteFiles[file] = state.localFiles[file]
			mu.Unlock()

			return nil
		})
	}

	uploadErr := p.Wait()

	// Reset pool for delete
	p = pool.New().WithMaxGoroutines(concurrency).WithErrors().WithContext(ctx)

	// Delete remote files that don't exist locally.
	for _, file := range state.toDelete() {
		// Skip reconciling index.md and manifest files for playgrounds
		if isExcluded(config.Kind, file) {
			continue
		}

		cli.PrintAux("🗑️  Deleting remote %s\n", file)

		if !config.Force && !cli.Confirm(fmt.Sprintf("File %s doesn't exist locally. Delete remotely?", file), "Yes", "No") {
			cli.PrintAux("Skipping...\n")
			continue
		}

		p.Go(func(ctx context.Context) error {
			err := cli.Client().DeleteContentFile(ctx, config.Kind, config.Name, file)

			if err != nil {
				return fmt.Errorf("couldn't delete remote content file %q: %w", file, err)
			}

			mu.Lock()
			delete(state.remoteFiles, file)
			mu.Unlock()

			return nil
		})
	}

	deleteErr := p.Wait()

	return errors.Join(uploadErr, deleteErr)
}

func listContentFilesRemote(ctx context.Context, client *api.Client, kind content.ContentKind, name string) (map[string]string, error) {
	remoteFiles, err := client.ListContentFiles(ctx, kind, name)
	if err != nil {
		return nil, fmt.Errorf("couldn't list remote content files: %w", err)
	}

	result := make(map[string]string)
	for _, file := range remoteFiles {
		result[file] = ""
	}

	return result, nil
}

func listContentFilesLocal(dir string) (map[string]string, error) {
	result := make(map[string]string)

	files, err := listFiles(dir)
	if err != nil {
		return nil, err
	}

	for _, abspath := range files {
		checksum, err := fileChecksum(abspath)
		if err != nil {
			return nil, err
		}

		relpath := strings.TrimPrefix(strings.TrimPrefix(abspath, dir), string(filepath.Separator))
		result[relpath] = checksum
	}

	return result, nil
}

func addWatchDirs(cli labcli.CLI, watcher *fsnotify.Watcher, state pushState) error {
	if !slices.Contains(watcher.WatchList(), state.dir) {
		if err := watcher.Add(state.dir); err != nil {
			return fmt.Errorf("couldn't add watch directory %s: %w", state.dir, err)
		}
	}

	dirs, err := listDirs(state.dir)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		if !slices.Contains(watcher.WatchList(), dir) {
			if err := watcher.Add(dir); err != nil {
				return fmt.Errorf("couldn't add watch directory %s: %w", dir, err)
			}
		}
	}

	cli.PrintAux("\n👀 Watching for changes in:\n")
	for _, dir := range watcher.WatchList() {
		cli.PrintAux("  - %s\n", dir)
	}
	cli.PrintAux("\n")

	return nil
}

func listDirs(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("couldn't list directory: %w", err)
	}

	var result []string
	for _, file := range files {
		fullPath := filepath.Join(dir, file.Name())
		if strings.Contains(fullPath, ".git") {
			continue
		}

		if file.IsDir() {
			result = append(result, fullPath)

			children, err := listDirs(fullPath)
			if err != nil {
				return nil, err
			}

			result = append(result, children...)
		}
	}

	return result, nil
}

func listFiles(dir string) ([]string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("couldn't list directory: %w", err)
	}

	var result []string
	for _, file := range files {
		if strings.HasPrefix(file.Name(), ".git") {
			continue
		}

		if file.IsDir() {
			children, err := listFiles(filepath.Join(dir, file.Name()))
			if err != nil {
				return nil, err
			}

			result = append(result, children...)
		} else {
			result = append(result, filepath.Join(dir, file.Name()))
		}
	}

	return result, nil
}

func fileChecksum(file string) (string, error) {
	f, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := md5.New() // no external actors, so md5 is fine
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func isExcluded(kind content.ContentKind, file string) bool {
	switch kind {
	case content.KindPlayground:
		return slices.Contains([]string{"index.md", "manifest.yaml", "manifest.yml"}, file)
	}

	return false
}
