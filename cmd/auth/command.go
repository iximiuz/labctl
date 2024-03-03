package auth

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/cliutil"
)

func NewCommand(cli cliutil.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Short: "Authenticate the current machine with iximiuz Labs",
		Use:   "auth <login|logout>",
	}

	cmd.AddCommand(
		newLoginCommand(cli),
		newLogoutCommand(cli),
		newWhoAmICommand(cli),
	)

	return cmd
}
