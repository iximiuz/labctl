package playground

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type updateOptions struct {
	file  string
	quiet bool
}

func newUpdateCommand(cli labcli.CLI) *cobra.Command {
	var opts updateOptions

	cmd := &cobra.Command{
		Use:   "update <playground-name> [flags]",
		Short: "Update an existing playground from a manifest file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if opts.file == "" {
				return labcli.NewStatusError(1, "--file flag is required")
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
		"Path to playground manifest file",
	)

	return cmd
}

func runUpdate(ctx context.Context, cli labcli.CLI, name string, opts *updateOptions) error {
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

	if manifest.Kind != "playground" {
		return fmt.Errorf("invalid manifest kind: %s", manifest.Kind)
	}

	req := api.UpdatePlaygroundRequest{
		Title:          manifest.Playground.Title,
		Description:    manifest.Playground.Description,
		Categories:     manifest.Playground.Categories,
		Access:         manifest.Playground.Access,
		Machines:       manifest.Playground.Machines,
		Tabs:           manifest.Playground.Tabs,
		InitTasks:      manifest.Playground.InitTasks,
		InitConditions: manifest.Playground.InitConditions,
		RegistryAuth:   manifest.Playground.RegistryAuth,
	}

	playground, err := cli.Client().UpdatePlayground(ctx, name, req)
	if err != nil {
		return fmt.Errorf("couldn't update playground: %w", err)
	}

	cli.PrintAux("Playground URL: %s\n", playground.PageURL)
	cli.PrintOut("%s\n", playground.Name)
	return nil
}
