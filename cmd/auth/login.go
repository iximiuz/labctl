package auth

// A typical login command that requests a one-time use URL from the auth endpoint and tries to open a browser with it.

import (
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/pkg/cliutil"
)

func newLoginCommand(cli cliutil.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in a user (you will be prompted to open a browser page with a one-time use URL)",
		Args:  cobra.NoArgs,
		RunE:  runLogin,
	}
	return cmd
}

func runLogin(cmd *cobra.Command, args []string) error {
	// failed opening browser. Copy the url (https://fly.io/app/auth/cli/523b1e60ee3a3631c0359ffc265640d0) into a browser and continue
	// Opening https://fly.io/app/auth/cli/523b1e60ee3a3631c0359ffc265640d0 ...
	return nil
}
