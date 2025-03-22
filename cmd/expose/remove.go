package expose

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewRemoveCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <playground> <id>",
		Aliases: []string{"rm"},
		Short:   "Un-expose a previously exposed port or shell by ID",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			playID := args[0]
			exposeID := args[1]
			return labcli.WrapStatusError(runRemove(cmd.Context(), cli, playID, exposeID))
		},
	}

	return cmd
}

func runRemove(ctx context.Context, cli labcli.CLI, playID, exposeID string) error {
	ports, err := cli.Client().ListPorts(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't list ports: %w", err)
	}

	for _, port := range ports {
		if port.ID == exposeID {
			if err := cli.Client().UnexposePort(ctx, playID, port.ID); err != nil {
				return fmt.Errorf("couldn't unexpose port: %w", err)
			}
			cli.PrintAux("Port %s (%s:%d) unexposed\n", port.ID, port.Machine, port.Number)
			return nil
		}
	}

	shells, err := cli.Client().ListShells(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't list shells: %w", err)
	}

	for _, shell := range shells {
		if shell.ID == exposeID {
			if err := cli.Client().UnexposeShell(ctx, playID, shell.ID); err != nil {
				return fmt.Errorf("couldn't unexpose shell: %w", err)
			}
			cli.PrintAux("Shell %s (%s@%s) unexposed\n", shell.ID, shell.User, shell.Machine)
			return nil
		}
	}

	return fmt.Errorf("couldn't find exposed port or shell with ID %s", exposeID)
}
