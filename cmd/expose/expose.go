package expose

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "expose <port|shell|list|remove> [playground] [target]",
		Aliases: []string{"e", "ex"},
		Short:   "Expose HTTP(s) ports and web terminals for a running playground",
		Long:    `Expose web UIs or HTTP(s) APIs running in a playground, or share access to the playground with a web terminal.`,
	}

	cmd.AddCommand(
		NewPortCommand(cli),
		NewShellCommand(cli),
		NewListCommand(cli),
		NewRemoveCommand(cli),
	)

	return cmd
}
