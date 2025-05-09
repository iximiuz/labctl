package playground

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/api"
	clicontent "github.com/iximiuz/labctl/cmd/content"
	"github.com/iximiuz/labctl/content"
	"github.com/iximiuz/labctl/internal/labcli"
)

type updateOptions struct {
	file string

	clicontent.DirOptions

	quiet bool

	force bool
}

func newUpdateCommand(cli labcli.CLI) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update <playground-name> [flags]",
		Short: "Update an existing playground from a manifest file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if opts.DirOptions.IsSet() && opts.file != "" {
				return fmt.Errorf("only one of --file or --dir can be provided")
			}

			return labcli.WrapStatusError(runUpdate(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print the playground name`,
	)
	flags.StringVarP(
		&opts.file,
		"file",
		"f",
		"",
		`Path to the playground manifest YAML file`,
	)
	flags.BoolVarP(
		&opts.force,
		"force",
		"",
		false,
		"Overwrite existing remote files with the local ones and delete remote files that don't exist locally without confirmation",
	)
	opts.DirOptions.AddDirFlag(
		flags,
		"Local directory with the manifest YAML file and possibly other content files to accompany the playground (default: $CWD/<playground-name>)",
	)

	return cmd
}

func runUpdate(ctx context.Context, cli labcli.CLI, name string, opts *updateOptions) error {
	if opts.file == "" {
		dir, err := opts.DirOptions.ContentDir(name)
		if err != nil {
			return fmt.Errorf("couldn't get the playground directory: %w", err)
		}

		if _, err := os.Stat(dir); err != nil {
			return fmt.Errorf("update failed: directory %s doesn't exist or is not accessible", dir)
		}

		for _, filename := range []string{"manifest.yaml", "manifest.yml"} {
			path := filepath.Join(dir, filename)
			if _, err := os.Stat(path); err != nil {
				continue
			}

			opts.file = path
			break
		}

		if opts.file == "" {
			return fmt.Errorf("update failed: no manifest file (manifest.yaml or manifest.yml) found in directory %s", dir)
		}
	}

	absFile, err := filepath.Abs(opts.file)
	if err != nil {
		return fmt.Errorf("couldn't get the absolute path of %s: %w", opts.file, err)
	}
	cli.PrintAux("Updating playground %s from %s\n", name, absFile)

	rawManifest, err := os.ReadFile(opts.file)
	if err != nil {
		return fmt.Errorf("failed to read manifest file: %w", err)
	}

	var manifest api.PlaygroundManifest
	if err := yaml.Unmarshal(rawManifest, &manifest); err != nil {
		return fmt.Errorf("failed to parse manifest file: %w", err)
	}

	if !manifest.Playground.HasAccessControl() && manifest.Playground.Access.Mode != "" {
		switch manifest.Playground.Access.Mode {
		case "private":
			// For backward compatibility
			manifest.Playground.AccessControl = api.PlaygroundAccessControl{
				CanList:  []string{"owner"},
				CanRead:  []string{"owner"},
				CanStart: []string{"owner"},
			}
		case "public":
			// For backward compatibility
			manifest.Playground.AccessControl = api.PlaygroundAccessControl{
				CanList:  []string{"anyone"},
				CanRead:  []string{"anyone"},
				CanStart: []string{"anyone"},
			}
		default:
			return fmt.Errorf("unsupported access mode: %s (only 'private' and 'public' are supported)", manifest.Playground.Access.Mode)
		}
	}

	if manifest.Kind != "playground" {
		return fmt.Errorf("invalid manifest kind: %s", manifest.Kind)
	}

	req := api.UpdatePlaygroundRequest{
		Title:          manifest.Title,
		Description:    manifest.Description,
		Cover:          manifest.Cover,
		Markdown:       manifest.Markdown,
		Categories:     manifest.Categories,
		Machines:       manifest.Playground.Machines,
		Tabs:           manifest.Playground.Tabs,
		InitTasks:      manifest.Playground.InitTasks,
		InitConditions: manifest.Playground.InitConditions,
		RegistryAuth:   manifest.Playground.RegistryAuth,
		AccessControl:  manifest.Playground.AccessControl,
	}

	playground, err := cli.Client().UpdatePlayground(ctx, name, req)
	if err != nil {
		return fmt.Errorf("couldn't update playground: %w", err)
	}

	if opts.DirOptions.IsSet() {
		config := clicontent.PushConfig{
			Kind:  content.KindPlayground,
			Name:  name,
			Dir:   filepath.Dir(opts.file),
			Force: opts.force,
		}

		err = clicontent.RunPushOnce(ctx, cli, config)
		if err != nil {
			return fmt.Errorf("couldn't update playground: %w", err)
		}
	}

	cli.PrintAux("Playground URL: %s\n", playground.PageURL)
	cli.PrintOut("%s\n", playground.Name)
	return nil
}
