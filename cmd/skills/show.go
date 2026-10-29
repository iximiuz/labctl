package skills

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	kit "github.com/iximiuz/labctl/skills"
)

type showOptions struct {
	agent string
}

func newShowCommand(cli labcli.CLI) *cobra.Command {
	var opts showOptions

	cmd := &cobra.Command{
		Use:   "show [flags] <skill-or-rule-name>",
		Short: "Print a skill (its SKILL.md) or a rule",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) > 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}

			var names []string
			for _, item := range kit.Default().Items() {
				names = append(names, item.Name+"\t"+string(item.Kind)+" ("+item.Bundle+")")
			}

			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runShow(cli, &opts, args[0]))
		},
	}

	cmd.Flags().StringVarP(
		&opts.agent,
		"agent",
		"a",
		"claude",
		"Print a rule in this AI agent's format",
	)
	_ = cmd.RegisterFlagCompletionFunc("agent", agentArgs)

	return cmd
}

func runShow(cli labcli.CLI, opts *showOptions, name string) error {
	k := kit.Default()

	if _, ok := k.Item(name); !ok {
		return labcli.NewStatusError(1, "unknown skill or rule %q\n\nHint: Use 'labctl skills list' to see the available ones", name)
	}

	content, err := k.Show(name, opts.agent)
	if err != nil {
		return err
	}

	if _, err := cli.OutputStream().Write(content); err != nil {
		return fmt.Errorf("couldn't print %s: %w", name, err)
	}

	return nil
}
