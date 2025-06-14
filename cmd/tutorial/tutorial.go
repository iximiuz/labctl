package tutorial

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tutorial <catalog|start|complete|stop>",
		Short:   "Follow tutorials from the comfort of your local command line",
		Aliases: []string{"tut", "tutorials"},
	}

	cmd.AddCommand(
		newCatalogCommand(cli),
		newStartCommand(cli),
		newCompleteCommand(cli),
		newStopCommand(cli),
	)

	return cmd
}
