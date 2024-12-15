package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type manifestOptions struct {
	name string
}

func newManifestCommand(cli labcli.CLI) *cobra.Command {
	var opts manifestOptions

	cmd := &cobra.Command{
		Use:   "manifest <playground-name>",
		Short: "View playground manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return labcli.WrapStatusError(runManifest(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runManifest(ctx context.Context, cli labcli.CLI, opts *manifestOptions) error {
	playground, err := cli.Client().GetPlayground(ctx, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	manifest := api.PlaygroundManifest{
		Kind:        "playground",
		Title:       playground.Title,
		Description: playground.Description,
		Categories:  playground.Categories,
		Playground: api.PlaygroundSpec{
			Access:         playground.Access,
			Machines:       playground.Machines,
			Tabs:           playground.Tabs,
			InitTasks:      playground.InitTasks,
			InitConditions: playground.InitConditions,
			RegistryAuth:   playground.RegistryAuth,
		},
	}

	bytes, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("couldn't marshal manifest: %w", err)
	}

	cli.PrintOut("%s", string(bytes))
	return nil
}
