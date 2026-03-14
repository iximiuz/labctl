package expose

import (
	"context"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/browser"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type portOptions struct {
	playID string
	port   string

	machine string

	https bool

	hostRewrite string
	pathRewrite string

	public bool

	open bool
}

func (o *portOptions) access() api.AccessMode {
	if o.public {
		return api.AccessPublic
	}
	return api.AccessPrivate
}

func (o *portOptions) protocol() string {
	if o.https {
		return "HTTPS"
	}
	return "HTTP"
}

func NewPortCommand(cli labcli.CLI) *cobra.Command {
	var opts portOptions

	cmd := &cobra.Command{
		Use:               "port <playground> <port>",
		Short:             "Expose an HTTP(s) service running in the playground",
		Args:              cobra.ExactArgs(2),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]
			opts.port = args[1]

			return labcli.WrapStatusError(runPort(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		"Target machine (default: the first machine in the playground)",
	)
	flags.BoolVarP(
		&opts.https,
		"https",
		"s",
		false,
		"Enable if the target service uses HTTPS (including self-signed certificates)",
	)
	flags.StringVar(
		&opts.hostRewrite,
		"host-rewrite",
		"",
		"Rewrite the host header passed to the target service",
	)
	flags.StringVar(
		&opts.pathRewrite,
		"path-rewrite",
		"",
		"Rewrite the path part of the URL passed to the target service",
	)
	flags.BoolVarP(
		&opts.public,
		"public",
		"p",
		false, "Make the exposed service publicly accessible",
	)
	flags.BoolVarP(
		&opts.open,
		"open",
		"o",
		false,
		"Open the exposed service in browser",
	)

	return cmd
}

func runPort(ctx context.Context, cli labcli.CLI, opts *portOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine, err = p.ResolveMachine(opts.machine); err != nil {
		return err
	}

	port, err := strconv.Atoi(opts.port)
	if err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("invalid port number (must be between 1 and 65535): %w", err)
	}

	resp, err := cli.Client().ExposePort(ctx, opts.playID, api.ExposePortRequest{
		Machine:     opts.machine,
		Number:      port,
		Access:      opts.access(),
		TLS:         opts.https,
		HostRewrite: opts.hostRewrite,
		PathRewrite: opts.pathRewrite,
	})
	if err != nil {
		return fmt.Errorf("couldn't expose port: %w", err)
	}

	cli.PrintAux("%s port %s:%d exposed as %s\n", opts.protocol(), resp.Machine, resp.Number, resp.URL)

	if opts.open {
		browser.OpenWithFallbackMessage(cli, resp.URL)
	}

	cli.PrintOut("%s\n", resp.URL)
	return nil
}
