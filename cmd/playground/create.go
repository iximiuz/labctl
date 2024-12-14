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

type createOptions struct {
	base string
	file string

	quiet bool
}

func newCreateCommand(cli labcli.CLI) *cobra.Command {
	var opts createOptions

	cmd := &cobra.Command{
		Use:   "create [flags]",
		Short: "Create a new playground from a base and a manifest file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			if opts.base == "" {
				return labcli.NewStatusError(1, "--base flag is required")
			}
			if opts.file == "" {
				return labcli.NewStatusError(1, "--file flag is required")
			}
			return labcli.WrapStatusError(runCreate(cmd.Context(), cli, &opts))
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
		&opts.base,
		"base",
		"b",
		"",
		"Base playground to use for the new playground",
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

func runCreate(ctx context.Context, cli labcli.CLI, opts *createOptions) error {
	absFile, err := filepath.Abs(opts.file)
	if err != nil {
		return fmt.Errorf("couldn't get the absolute path of %s: %w", opts.file, err)
	}
	cli.PrintAux("Creating playground from %s\n", absFile)

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

	req := api.CreatePlaygroundRequest{
		Base:           opts.base,
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

	playground, err := cli.Client().CreatePlayground(ctx, req)
	if err != nil {
		return fmt.Errorf("couldn't create playground: %w", err)
	}

	cli.PrintAux("Playground URL: %s\n", playground.PageURL)
	cli.PrintOut("%s\n", playground.Name)
	return nil
}
