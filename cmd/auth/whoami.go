package auth

import (
	"context"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"
)

func newWhoAmICommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Print the current user info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runWhoAmI(cmd.Context(), cli))
		},
	}
	return cmd
}

func runWhoAmI(ctx context.Context, cli labcli.CLI) error {
	if cli.Config().SessionID == "" || cli.Config().AccessToken == "" {
		cli.PrintErr("Not logged in. Use 'labctl auth login' to log in.\n")
		return nil
	}

	me, err := cli.Client().GetAccount(ctx)
	if err != nil {
		return err
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(me); err != nil {
		return err
	}

	return nil
}
