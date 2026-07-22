package auth

import (
	"context"
	"errors"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

func newSigninURLCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:    "signin-url",
		Short:  "Generate a one-time sign-in URL for the current user",
		Args:   cobra.NoArgs,
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runSigninURL(cmd.Context(), cli))
		},
	}
	return cmd
}

func runSigninURL(ctx context.Context, cli labcli.CLI) error {
	if cli.Config().SessionID == "" || cli.Config().AccessToken == "" {
		cli.PrintErr("Not logged in. Use 'labctl auth login' to log in.\n")
		return nil
	}

	signinURL, err := cli.Client().GenerateSigninURL(ctx)
	if err != nil {
		if errors.Is(err, api.ErrAuthenticationRequired) {
			cli.PrintErr("Authentication session expired. Please log in again: labctl auth login\n")
			return nil
		}

		return err
	}

	cli.PrintOut("%s\n", signinURL.URL)

	return nil
}
