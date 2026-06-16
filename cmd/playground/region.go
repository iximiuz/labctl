package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type regionOptions struct {
	region string
}

func newRegionCommand(cli labcli.CLI) *cobra.Command {
	var opts regionOptions

	cmd := &cobra.Command{
		Use:   "region [region]",
		Short: fmt.Sprintf("Show or set your preferred region for new playgrounds (one of: %s)", api.KnownRegionsString()),
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return api.KnownRegions, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return labcli.WrapStatusError(runShowPlaygroundRegion(cmd.Context(), cli))
			}

			opts.region = args[0]

			return labcli.WrapStatusError(runSetPlaygroundRegion(cmd.Context(), cli, &opts))
		},
	}

	return cmd
}

func runShowPlaygroundRegion(ctx context.Context, cli labcli.CLI) error {
	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return fmt.Errorf("couldn't get the current user: %w", err)
	}

	if me.PreferredRegion == "" {
		return labcli.NewStatusError(1, "no preferred region is set")
	}

	cli.PrintOut("%s\n", me.PreferredRegion)

	return nil
}

func runSetPlaygroundRegion(ctx context.Context, cli labcli.CLI, opts *regionOptions) error {
	if !api.IsKnownRegion(opts.region) {
		return fmt.Errorf("invalid region %q (valid regions: %s)", opts.region, api.KnownRegionsString())
	}

	cli.PrintAux("Setting preferred region to %s...\n", opts.region)

	me, err := cli.Client().SetPreferredRegion(ctx, opts.region)
	if err != nil {
		return fmt.Errorf("couldn't set the preferred region: %w", err)
	}

	cli.PrintAux("Preferred region is now set to %s.\n", me.PreferredRegion)

	return nil
}
