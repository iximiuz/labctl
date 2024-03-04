package auth

import (
	"context"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/ssh"
)

func newLogoutCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Log out the current user by deleting the current CLI session",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runLogout(cmd.Context(), cli))
		},
	}
	return cmd
}

func runLogout(ctx context.Context, cli labcli.CLI) error {
	if cli.Config().SessionID == "" || cli.Config().AccessToken == "" {
		cli.PrintAux("No active session found. You are already logged out.\n")
		return nil
	}

	// TODO: Check HTTP 404 explicitly to handle the case when the session is already deleted.
	if err := cli.Client().DeleteSession(ctx, cli.Config().SessionID); err != nil {
		return err
	}

	if err := ssh.RemoveIdentity(cli.Config().SSHDirPath); err != nil {
		slog.Warn("Failed to remove SSH identity file: %v", err)
	}

	cli.Config().SessionID = ""
	cli.Config().AccessToken = ""
	if err := cli.Config().Dump(); err != nil {
		return err
	}

	cli.PrintAux("Logged out successfully.\n")

	return nil
}
