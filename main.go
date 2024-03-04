package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/moby/term"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/cmd/auth"
	"github.com/iximiuz/labctl/cmd/portforward"
	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/internal/api"
	"github.com/iximiuz/labctl/internal/config"
	"github.com/iximiuz/labctl/internal/labcli"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type configOverrides struct {
	endpoint string
}

func main() {
	stdin, stdout, stderr := term.StdStreams()
	cli := labcli.NewCLI(stdin, stdout, stderr)

	var (
		logLevel  string
		overrides configOverrides
	)

	cmd := &cobra.Command{
		Use:     "labctl <auth|playgrounds|port-forward|ssh|...>",
		Short:   "labctl - iximiuz Labs command line interface.",
		Version: fmt.Sprintf("%s (built: %s commit: %s)", version, date, commit),
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setLogLevel(cli, logLevel)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			loadConfigOrFail(cli, overrides)

			cli.SetClient(api.NewClient(api.ClientOptions{
				BaseURL:     cli.Config().APIBaseURL,
				SessionID:   cli.Config().SessionID,
				AccessToken: cli.Config().AccessToken,
				UserAgent:   fmt.Sprintf("labctl/%s", cmd.Version),
			}))
		},
	}
	cmd.SetOut(cli.OutputStream())
	cmd.SetErr(cli.ErrorStream())

	cmd.AddCommand(
		auth.NewCommand(cli),
		portforward.NewCommand(cli),
		ssh.NewCommand(cli),
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
	flags.StringVar(
		&overrides.endpoint,
		"endpoint",
		"",
		"iximiuz Labs API endpoint URL",
	)

	if err := cmd.Execute(); err != nil {
		if sterr, ok := err.(labcli.StatusError); ok {
			cli.PrintErr("labctl: %s\n", sterr)
			os.Exit(sterr.Code())
		}

		// Hopefully, only usage errors.
		slog.Debug("Exit error: %s", err)
		os.Exit(1)
	}
}

func loadConfigOrFail(cli labcli.CLI, overrides configOverrides) {
	configPath, err := config.ConfigFilePath()
	if err != nil {
		cli.PrintErr("Unable to determine config path: %s\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		cli.PrintErr("Unable to load config: %s\n", err)
		cli.PrintErr("Is %s corrupted?\n", configPath)
		cfg = config.Default(configPath)
	}

	if overrides.endpoint != "" {
		cfg.APIBaseURL = overrides.endpoint
	}

	cli.SetConfig(cfg)
}

func setLogLevel(cli labcli.CLI, logLevel string) {
	var level slog.Level
	if err := level.UnmarshalText([]byte(logLevel)); err != nil {
		cli.PrintErr("Unable to parse log level: %s\n", logLevel)
		os.Exit(1)
	}

	slog.SetLogLoggerLevel(level)
}
