package auth

import (
	"context"
	"time"

	"github.com/briandowns/spinner"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/cliutil"
)

func newLoginCommand(cli cliutil.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in a user (you will be prompted to open a browser page with a one-time use URL)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cliutil.WrapStatusError(runLogin(cmd.Context(), cli))
		},
	}
	return cmd
}

func runLogin(ctx context.Context, cli cliutil.CLI) error {
	ses, err := cli.Client().CreateSession(ctx)
	if err != nil {
		return err
	}

	accessToken := ses.AccessToken
	cli.Client().SetCredentials(ses.ID, accessToken)

	cli.PrintAux("Opening %s in your browser...\n", ses.AuthURL)

	if err := open.Run(ses.AuthURL); err != nil {
		cli.PrintAux("Failed opening the browser. Copy the above URL into a browser manually and follow the instructions on the page.\n")
	}

	cli.PrintAux("\n")

	s := spinner.New(spinner.CharSets[39], 300*time.Millisecond)
	s.Writer = cli.AuxStream()
	s.Prefix = "Waiting for the session to be authorized... "
	s.Start()

	for ctx.Err() == nil {
		if ses, err := cli.Client().GetSession(ctx, ses.ID); err == nil && ses.Authenticated {
			s.FinalMSG = "Waiting for the session to be authorized... Done.\n"
			s.Stop()

			cli.Config().SessionID = ses.ID
			cli.Config().AccessToken = accessToken
			if err := cli.Config().Dump(); err != nil {
				return err
			}

			cli.PrintAux("\nSession authorized. You can now use labctl commands.\n")
			return nil
		}

		time.Sleep(2 * time.Second)
	}

	return nil
}
