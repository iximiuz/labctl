package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/moby/term"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/auth"
	"github.com/iximiuz/labctl/cmd/portforward"
	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/cliutil"
	"github.com/iximiuz/labctl/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func main() {
	stdin, stdout, stderr := term.StdStreams()
	cli := cliutil.NewCLI(stdin, stdout, stderr)

	versionStr := fmt.Sprintf("%s (built: %s commit: %s)", version, date, commit)

	cfg, err := config.Load()
	if err != nil {
		slog.Debug("Unable to load config: %s", err)
		cfg = config.Default()
	}
	cli.SetConfig(cfg)

	cli.SetClient(api.NewClient(api.ClientOptions{
		BaseURL:     cfg.APIBaseURL,
		SessionID:   cfg.SessionID,
		AccessToken: cfg.AccessToken,
		UserAgent:   fmt.Sprintf("labctl/%s", versionStr),
	}))

	var logLevel string
	logrus.SetOutput(cli.ErrorStream())

	cmd := &cobra.Command{
		Short:   "This is labctl, the iximiuz Labs command line interface.",
		Use:     "labctl <auth|playgrounds|port-forward|ssh> [flags]",
		Version: versionStr,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setLogLevel(cli, logLevel)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true
		},
	}
	cmd.SetOut(cli.OutputStream())
	cmd.SetErr(cli.ErrorStream())

	cmd.AddCommand(
		auth.NewCommand(cli),
		portforward.NewCommand(cli),
		// TODO: other commands
	)

	flags := cmd.PersistentFlags()
	flags.SetInterspersed(false) // Instead of relying on --

	flags.StringVarP(
		&logLevel,
		"log-level",
		"l",
		"info",
		`log level for labctl ("debug" | "info" | "warn" | "error" | "fatal")`,
	)

	if err := cmd.Execute(); err != nil {
		if sterr, ok := err.(cliutil.StatusError); ok {
			cli.PrintErr("labctl: %s\n", sterr)
			os.Exit(sterr.Code())
		}

		// Hopefully, only usage errors.
		logrus.Debugf("Exit error: %s", err)
		os.Exit(1)
	}
}

func setLogLevel(cli cliutil.CLI, logLevel string) {
	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		cli.PrintErr("Unable to parse log level: %s\n", logLevel)
		os.Exit(1)
	}
	logrus.SetLevel(lvl)
}
