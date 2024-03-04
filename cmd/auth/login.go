package auth

import (
	"context"
	"fmt"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/ssh"
)

const (
	loginSessionTimeout = 10 * time.Minute
)

func newLoginCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in as a Labs user (you will be prompted to open a browser page with a one-time use URL)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runLogin(cmd.Context(), cli))
		},
	}
	return cmd
}

func runLogin(ctx context.Context, cli labcli.CLI) error {
	ses, err := cli.Client().CreateSession(ctx)
	if err != nil {
		return fmt.Errorf("couldn't start a session: %w", err)
	}

	accessToken := ses.AccessToken
	cli.Client().SetCredentials(ses.ID, accessToken)

	cli.PrintAux("Opening %s in your browser...\n", ses.AuthURL)

	if err := open.Run(ses.AuthURL); err != nil {
		cli.PrintAux("Couldn't open the browser. Copy the above URL into a browser manually and follow the instructions on the page.\n")
	}

	cli.PrintAux("\n")

	s := spinner.New(spinner.CharSets[39], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for the session to be authorized... "
	s.Start()

	ctx, cancel := context.WithTimeout(ctx, loginSessionTimeout)
	defer cancel()

	for ctx.Err() == nil {
		if ses, err := cli.Client().GetSession(ctx, ses.ID); err == nil && ses.Authenticated {
			s.FinalMSG = "Waiting for the session to be authorized... Done.\n"
			s.Stop()

			cli.Config().SessionID = ses.ID
			cli.Config().AccessToken = accessToken
			if err := cli.Config().Dump(); err != nil {
				return fmt.Errorf("couldn't save the credentials to the config file: %w", err)
			}

			if err := ssh.GenerateIdentity(cli.Config().SSHDirPath); err != nil {
				return fmt.Errorf("couldn't generate SSH identity in %s: %w", cli.Config().SSHDirPath, err)
			}

			cli.PrintAux("\nSession authorized. You can now use labctl commands.\n")
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}
