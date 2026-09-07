package shellgym

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "shell-gym <start|stop> [shell-gym-name]",
		Aliases: []string{"gym", "shell-gyms"},
		Short:   "Train with Shell Gyms from the comfort of your terminal",
	}

	cmd.AddCommand(
		newStartCommand(cli),
		newStopCommand(cli),
	)

	return cmd
}
