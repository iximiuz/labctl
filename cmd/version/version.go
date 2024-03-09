package version

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func NewCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: `Print the version of labctl`,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.PrintOut(cli.Version() + "\n")
			return nil
		},
	}

	return cmd
}
