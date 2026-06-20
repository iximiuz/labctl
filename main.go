package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"github.com/moby/term"
	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	apicmd "github.com/iximiuz/labctl/cmd/api"
	"github.com/iximiuz/labctl/cmd/auth"
	"github.com/iximiuz/labctl/cmd/challenge"
	"github.com/iximiuz/labctl/cmd/content"
	"github.com/iximiuz/labctl/cmd/course"
	"github.com/iximiuz/labctl/cmd/cp"
	"github.com/iximiuz/labctl/cmd/expose"
	"github.com/iximiuz/labctl/cmd/ide"
	"github.com/iximiuz/labctl/cmd/kubeproxy"
	"github.com/iximiuz/labctl/cmd/playground"
	"github.com/iximiuz/labctl/cmd/portforward"
	"github.com/iximiuz/labctl/cmd/ssh"
	"github.com/iximiuz/labctl/cmd/sshproxy"
	"github.com/iximiuz/labctl/cmd/tui"
	"github.com/iximiuz/labctl/cmd/tutorial"
	versioncmd "github.com/iximiuz/labctl/cmd/version"
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
	cli := labcli.NewCLI(
		stdin, stdout, stderr,
		fmt.Sprintf("%s (built: %s commit: %s)", version, date, commit),
	)

	var (
		logLevel  string
		overrides configOverrides
	)

	cmd := &cobra.Command{
		Use:     "labctl <auth|playgrounds|port-forward|ssh|...>",
		Short:   "labctl - iximiuz Labs command line interface.",
		Version: cli.Version(),
		// Bare `labctl` launches the TUI when LABCTL_TUI is truthy, otherwise
		// prints help as before.
		RunE: func(cmd *cobra.Command, args []string) error {
			if tuiDefaultEnabled() {
				return labcli.WrapStatusError(tui.Run(cli))
			}
			return cmd.Help()
		},
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			setLogLevel(cli, logLevel)
			cmd.SilenceUsage = true
			cmd.SilenceErrors = true

			loadConfigOrFail(cli, overrides)

			cli.SetClient(api.NewClient(api.ClientOptions{
				BaseURL:     cli.Config().BaseURL,
				APIBaseURL:  cli.Config().APIBaseURL,
				SessionID:   cli.Config().SessionID,
				AccessToken: cli.Config().AccessToken,
				UserAgent:   fmt.Sprintf("labctl/%s", cli.Version()),
			}))
		},
	}
	cmd.SetOut(cli.OutputStream())
	cmd.SetErr(cli.ErrorStream())

	cmd.AddCommand(
		apicmd.NewCommand(cli),
		auth.NewCommand(cli),
		challenge.NewCommand(cli),
		content.NewCommand(cli),
		course.NewCommand(cli),
		cp.NewCommand(cli),
		expose.NewCommand(cli),
		ide.NewCommand(cli),
		kubeproxy.NewCommand(cli),
		playground.NewCommand(cli),
		portforward.NewCommand(cli),
		ssh.NewCommand(cli),
		sshproxy.NewCommand(cli),
		tui.NewCommand(cli),
		tutorial.NewCommand(cli),
		versioncmd.NewCommand(cli),
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
		if sterr := (labcli.StatusError{}); errors.As(err, &sterr) {
			cli.PrintErr("labctl: %s\n", err.Error())
			os.Exit(sterr.Code())
		}

		// Hopefully, only usage errors.
		slog.Debug("Exit error: " + err.Error())
		os.Exit(1)
	}
}

// tuiDefaultEnabled reports whether bare `labctl` should launch the TUI, based
// on the LABCTL_TUI env var (e.g. 1, true, yes). Unset or a falsy value (0,
// false) keeps the default help behavior.
func tuiDefaultEnabled() bool {
	v, ok := os.LookupEnv("LABCTL_TUI")
	if !ok {
		return false
	}
	if b, err := strconv.ParseBool(v); err == nil {
		return b
	}
	return v != "" // tolerate non-canonical truthy values like "yes"/"on"
}

func loadConfigOrFail(cli labcli.CLI, overrides configOverrides) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		cli.PrintErr("Unable to determine home directory: %s\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(homeDir)
	if err != nil {
		cli.PrintErr("Unable to load config: %s\n", err)
		cli.PrintErr("Is %s corrupted?\n", config.ConfigFilePath(homeDir))
		cli.PrintErr("Using default config...\n")
		cfg = config.Default(homeDir)
	}

	if overrides.endpoint != "" {
		cfg.BaseURL = overrides.endpoint
		cfg.APIBaseURL = overrides.endpoint + "/api"
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
