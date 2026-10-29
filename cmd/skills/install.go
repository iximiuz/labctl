package skills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	kit "github.com/iximiuz/labctl/skills"
)

type installOptions struct {
	targetOptions
	agentOptions

	force bool
}

func newInstallCommand(cli labcli.CLI) *cobra.Command {
	var opts installOptions

	cmd := &cobra.Command{
		Use:   "install [flags] [bundle...]",
		Short: "Install or update the skills and rules for the chosen AI agents",
		Long: `Install or update the skills and rules for the chosen AI agents in the
project (or in the home directory with --user).

Bundles:
` + bundlesHelp() + `
Without arguments, the command updates what is already installed or, on the
first run, installs all bundles available for the target. Agents and bundles
given later are added to the installed ones. Run the command again after
upgrading labctl to update the skills. Locally modified files are kept unless
--force is given.`,
		Example: `  # Install everything into the current project (a content repository):
  labctl skills install --agent claude

  # Install only the playground skills for all your projects:
  labctl skills install playgrounds --user --agent cursor

  # Update the installed skills after upgrading labctl:
  labctl skills install`,
		ValidArgsFunction: bundleArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runInstall(cli, &opts, args))
		},
	}

	flags := cmd.Flags()

	opts.targetOptions.addFlags(flags)
	opts.agentOptions.addFlags(flags, "AI agents to install the skills for")
	_ = cmd.RegisterFlagCompletionFunc("agent", agentArgs)

	flags.BoolVarP(
		&opts.force,
		"force",
		"f",
		false,
		"Overwrite locally modified files (and files that labctl didn't install)",
	)

	return cmd
}

func runInstall(cli labcli.CLI, opts *installOptions, names []string) error {
	t, err := opts.target(cli)
	if err != nil {
		return err
	}

	k := kit.Default()

	manifest, _, err := kit.LoadManifest(t)
	if err != nil {
		return err
	}

	agents := opts.agents
	if len(agents) == 0 && len(manifest.Agents) == 0 {
		if agents, err = chooseAgents(cli); err != nil {
			return err
		}
	}

	agents, err = kit.ResolveAgents(union(manifest.Agents, agents))
	if err != nil {
		return err
	}

	if len(names) == 0 && len(manifest.Bundles) == 0 {
		for _, b := range k.Bundles() {
			if t.Scope == kit.ScopeProject || !b.ProjectOnly {
				names = append(names, b.Name)
			}
		}
	}

	bundles, err := k.ResolveBundles(union(manifest.Bundles, names))
	if err != nil {
		return err
	}

	if t.Scope == kit.ScopeUser {
		for _, name := range bundles {
			if b, _ := k.Bundle(name); b.ProjectOnly {
				return fmt.Errorf("the %s bundle can be installed only into a project: its rules apply to the project's content folders (challenges/, tutorials/, ...)", name)
			}
		}
	}

	changes, err := k.Sync(t, kit.SyncOptions{
		Agents:  agents,
		Bundles: bundles,
		Version: versionOf(cli),
		Force:   opts.force,
	})
	if err != nil {
		return fmt.Errorf("couldn't install skills: %w", err)
	}

	printChanges(cli, changes)

	var skills, rules int
	for _, item := range k.Items() {
		if !contains(bundles, item.Bundle) {
			continue
		}
		if item.Kind == kit.KindSkill {
			skills++
		} else if t.Scope == kit.ScopeProject {
			rules++
		}
	}

	bundleWord := "bundle"
	if len(bundles) > 1 {
		bundleWord = "bundles"
	}

	what := fmt.Sprintf("%s %s (%d skills, %d rules) for %s", joinWords(bundles), bundleWord, skills, rules, agentTitles(agents))
	skipped := countChanges(changes, kit.ActionSkip)

	switch {
	case len(changes) > skipped:
		cli.PrintAux("Installed the %s in %s.\n", what, t.Root)
	case skipped == 0:
		cli.PrintAux("The %s in %s are up to date.\n", what, t.Root)
	}

	if skipped > 0 {
		cli.PrintAux("Skipped %d file(s) that labctl didn't write or that were modified locally - see above.\n", skipped)
	}

	return nil
}
