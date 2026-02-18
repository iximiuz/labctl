package course

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "course <start|stop>",
		Short:   "Start and stop course lessons from the comfort of your terminal",
		Aliases: []string{"courses"},
	}

	cmd.AddCommand(
		newStartCommand(cli),
		newStopCommand(cli),
	)

	return cmd
}
