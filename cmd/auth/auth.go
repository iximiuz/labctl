package auth

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth <login|logout|token|whoami>",
		Short: "Authenticate the current CLI session with iximiuz Labs",
	}

	cmd.AddCommand(
		newLoginCommand(cli),
		newLogoutCommand(cli),
		newSigninURLCommand(cli),
		newTokenCommand(cli),
		newWhoAmICommand(cli),
	)

	return cmd
}
