package auth

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

const defaultTokenExpiresInDays = 90

type tokenOptions struct {
	name      string
	scope     []string
	expiresIn int

	quiet bool
}

func newTokenCommand(cli labcli.CLI) *cobra.Command {
	var opts tokenOptions

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Mint an access token for the Labs MCP server (handy for AI tools running in remote VMs and sandboxes)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			cli.SetQuiet(opts.quiet)

			return labcli.WrapStatusError(runToken(cmd.Context(), cli, opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVar(
		&opts.name,
		"name",
		defaultTokenName(),
		`Human-readable token name shown in Account -> Connected apps`,
	)
	flags.StringSliceVar(
		&opts.scope,
		"scope",
		api.OAuthTokenDefaultScopes,
		`Scope to grant the token; multiple --scope flags can be used. Valid scopes: `+
			strings.Join(api.OAuthTokenScopes, ", ")+` (the author:* scopes require a pro account)`,
	)
	flags.IntVar(
		&opts.expiresIn,
		"expires-in",
		defaultTokenExpiresInDays,
		`Token lifetime in days (1..365)`,
	)
	flags.BoolVarP(
		&opts.quiet,
		"quiet",
		"q",
		false,
		`Only print the token`,
	)

	return cmd
}

func runToken(ctx context.Context, cli labcli.CLI, opts tokenOptions) error {
	if cli.Config().SessionID == "" || cli.Config().AccessToken == "" {
		cli.PrintErr("Not logged in. Use 'labctl auth login' to log in.\n")
		return nil
	}

	for _, scope := range opts.scope {
		if !slices.Contains(api.OAuthTokenScopes, scope) {
			return labcli.NewStatusError(1,
				"Unknown scope %q. Valid scopes: %s",
				scope, strings.Join(api.OAuthTokenScopes, ", "),
			)
		}
	}

	token, err := cli.Client().CreateOAuthToken(ctx, api.CreateOAuthTokenRequest{
		Name:          opts.name,
		Scope:         opts.scope,
		ExpiresInDays: opts.expiresIn,
	})
	if err != nil {
		if errors.Is(err, api.ErrAuthenticationRequired) {
			cli.PrintErr("Authentication session expired. Please log in again: labctl auth login\n")
			return nil
		}

		return err
	}

	cli.PrintOut("%s\n", token.AccessToken)

	siteURL := cli.Config().BaseURL

	cli.PrintAux("\n")
	cli.PrintAux("Token %q (scope: %s) expires at %s.\n",
		token.Name, strings.Join(token.Scope, " "), token.ExpiresAt)
	cli.PrintAux("It's shown only once - store it securely. You can revoke it at %s/account (Connected apps).\n", siteURL)
	cli.PrintAux("\n")
	cli.PrintAux("For instance, to connect Claude Code to the Labs MCP server:\n")
	cli.PrintAux("\n")
	cli.PrintAux("    claude mcp add --transport http ixlabs %s/mcp --header \"Authorization: Bearer %s\"\n",
		siteURL, token.AccessToken)
	cli.PrintAux("\n")

	return nil
}

func defaultTokenName() string {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		return "labctl"
	}

	return "labctl@" + hostname
}
