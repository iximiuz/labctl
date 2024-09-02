package challenge

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "challenge <list|start|stop> [challenge-name]",
		Aliases: []string{"ch", "challenges"},
		Short:   "Solve DevOps challenges from the comfort of your terminal",
	}

	cmd.AddCommand(
		newCatalogCommand(cli),
		newListCommand(cli),
		newStartCommand(cli),
		newCompleteCommand(cli),
		newStopCommand(cli),
	)

	return cmd
}
