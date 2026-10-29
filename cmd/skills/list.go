package skills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	kit "github.com/iximiuz/labctl/skills"
)

type listOptions struct {
	targetOptions
	agentOptions
}

func newListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list [flags]",
		Aliases: []string{"ls"},
		Short:   "List the available skills and rules and their install status",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runList(cli, &opts))
		},
	}

	opts.targetOptions.addFlags(cmd.Flags())
	opts.agentOptions.addFlags(cmd.Flags(), "Show the status for these AI agents (default: the installed ones)")
	_ = cmd.RegisterFlagCompletionFunc("agent", agentArgs)

	return cmd
}

func runList(cli labcli.CLI, opts *listOptions) error {
	t, err := opts.target(cli)
	if err != nil {
		return err
	}

	k := kit.Default()

	manifest, _, err := kit.LoadManifest(t)
	if err != nil {
		return err
	}

	agents := manifest.Agents
	if len(opts.agents) > 0 {
		if agents, err = kit.ResolveAgents(opts.agents); err != nil {
			return err
		}
	}

	status, err := k.Status(t, agents)
	if err != nil {
		return err
	}

	switch {
	case len(manifest.Files) > 0:
		cli.PrintAux("Installed in %s for %s by labctl %s\n\n", t.Root, agentTitles(manifest.Agents), manifest.Version)
	default:
		cli.PrintAux("Nothing is installed in %s yet - run 'labctl skills install'.\n\n", t.Root)
	}

	printer := labcli.NewSliceTablePrinter[kit.Item](
		cli.OutputStream(),
		[]string{"NAME", "KIND", "BUNDLE", "STATUS"},
		func(item kit.Item) []string {
			return []string{item.Name, string(item.Kind), item.Bundle, string(status[item.Name])}
		},
	)
	if err := printer.Print(k.Items()); err != nil {
		return fmt.Errorf("couldn't print the list: %w", err)
	}
	printer.Flush()

	return nil
}
