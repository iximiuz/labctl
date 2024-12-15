package playground

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "playground <list|start|stop> [playground-name]",
		Aliases: []string{"p", "playgrounds"},
		Short:   "List, start and stop playgrounds",
	}

	cmd.AddCommand(
		newListCommand(cli),
		newCatalogCommand(cli),
		newStartCommand(cli),
		newStopCommand(cli),
		newMachinesCommand(cli),
		newCreateCommand(cli),
		newViewCommand(cli),
		newUpdateCommand(cli),
	)

	return cmd
}
