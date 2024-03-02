package auth

import (
	"context"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/iximiuz/labctl/internal/cliutil"
)

func newWhoAmICommand(cli cliutil.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Print the current user info",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.WrapStatusError(runWhoAmI(cmd.Context(), cli))
		},
	}
	return cmd
}

func runWhoAmI(ctx context.Context, cli cliutil.CLI) error {
	if cli.Config().SessionID == "" || cli.Config().AccessToken == "" {
		cli.PrintErr("Not logged in. Use 'labctl auth login' to log in.\n")
		return nil
	}

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		return err
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(me); err != nil {
		return err
	}

	return nil
}
