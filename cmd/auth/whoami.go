package auth

import (
	"context"
	"errors"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newWhoAmICommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "whoami",
		Aliases: []string{"who", "me"},
		Short:   "Print the current user info",
		Args:    cobra.NoArgs,
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

	me, err := cli.Client().GetMe(ctx)
	if err != nil {
		if errors.Is(err, api.ErrAuthenticationRequired) {
			cli.PrintErr("Authentication session expired. Please log in again: labctl auth login\n")
			return nil
		}

		return err
	}

	if err := yaml.NewEncoder(cli.OutputStream()).Encode(me); err != nil {
		return err
	}

	return nil
}
