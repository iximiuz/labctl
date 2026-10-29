package skills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	kit "github.com/iximiuz/labctl/skills"
)

type uninstallOptions struct {
	targetOptions
	agentOptions

	force bool
}

func newUninstallCommand(cli labcli.CLI) *cobra.Command {
	var opts uninstallOptions

	cmd := &cobra.Command{
		Use:   "uninstall [flags] [bundle...]",
		Short: "Remove the installed skills and rules (everything, some bundles, or some agents)",
		Example: `  # Remove everything labctl installed in the current project:
  labctl skills uninstall

  # Stop installing the skills for Codex:
  labctl skills uninstall --agent codex`,
		ValidArgsFunction: bundleArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runUninstall(cli, &opts, args))
		},
	}

	flags := cmd.Flags()

	opts.targetOptions.addFlags(flags)
	opts.agentOptions.addFlags(flags, "Remove the skills only for these AI agents")
	_ = cmd.RegisterFlagCompletionFunc("agent", agentArgs)

	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Remove locally modified files too",
	)

	return cmd
}

func runUninstall(cli labcli.CLI, opts *uninstallOptions, bundles []string) error {
	t, err := opts.target(cli)
	if err != nil {
		return err
	}

	k := kit.Default()

	manifest, exists, err := kit.LoadManifest(t)
	if err != nil {
		return err
	}
	if !exists {
		cli.PrintAux("Nothing to uninstall: labctl didn't install any skills in %s.\n", t.Root)
		return nil
	}

	if len(opts.agents) > 0 && len(bundles) > 0 {
		return fmt.Errorf("uninstall either bundles or agents (--agent), not both at once")
	}

	agents, keep := manifest.Agents, manifest.Bundles

	for _, a := range opts.agents {
		if !contains(manifest.Agents, a) {
			return fmt.Errorf("the skills are not installed for %q in %s", a, t.Root)
		}
	}
	agents = without(agents, opts.agents)

	for _, b := range bundles {
		if !contains(manifest.Bundles, b) {
			return fmt.Errorf("the %s bundle is not installed in %s", b, t.Root)
		}
	}
	keep = without(keep, bundles)

	for _, name := range keep {
		b, _ := k.Bundle(name)
		for _, req := range b.Requires {
			if !contains(keep, req) {
				return fmt.Errorf("the %s bundle needs the %s bundle - uninstall it too", name, req)
			}
		}
	}

	if (len(opts.agents) == 0 && len(bundles) == 0) || len(agents) == 0 || len(keep) == 0 {
		agents, keep = nil, nil
	}

	changes, err := k.Sync(t, kit.SyncOptions{
		Agents:  agents,
		Bundles: keep,
		Version: versionOf(cli),
		Force:   opts.force,
	})
	if err != nil {
		return fmt.Errorf("couldn't uninstall skills: %w", err)
	}

	printChanges(cli, changes)

	cli.PrintAux("Removed %d file(s) from %s.\n", countChanges(changes, kit.ActionRemove), t.Root)
	if n := countChanges(changes, kit.ActionSkip); n > 0 {
		cli.PrintAux("Kept %d locally modified file(s) - see above.\n", n)
	}

	return nil
}
