package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type viewOptions struct {
	name string
}

func newViewCommand(cli labcli.CLI) *cobra.Command {
	var opts viewOptions

	cmd := &cobra.Command{
		Use:   "view <playground-name>",
		Short: "View playground manifest",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.name = args[0]
			return labcli.WrapStatusError(runView(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runView(ctx context.Context, cli labcli.CLI, opts *viewOptions) error {
	playground, err := cli.Client().GetPlayground(ctx, opts.name)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	manifest := api.PlaygroundManifest{
		Kind: "playground",
		Playground: api.PlaygroundSpec{
			Name:           playground.Name,
			Title:          playground.Title,
			Description:    playground.Description,
			Categories:     playground.Categories,
			Access:         playground.Access,
			Tabs:           playground.Tabs,
			Machines:       playground.Machines,
			InitTasks:      playground.InitTasks,
			InitConditions: playground.InitConditions,
		},
	}

	bytes, err := yaml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("couldn't marshal manifest: %w", err)
	}

	cli.PrintOut("%s", string(bytes))
	return nil
}
