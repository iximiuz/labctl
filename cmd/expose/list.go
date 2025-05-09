package expose

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
)

type listOptions struct {
	kind string
}

func NewListCommand(cli labcli.CLI) *cobra.Command {
	var opts listOptions

	cmd := &cobra.Command{
		Use:     "list <playground>",
		Aliases: []string{"ls"},
		Short:   "List all exposed HTTP(s) ports and web terminals",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			playID := args[0]
			return labcli.WrapStatusError(runList(cmd.Context(), cli, playID, &opts))
		},
	}

	flags := cmd.Flags()
	flags.StringVar(
		&opts.kind,
		"kind",
		"",
		"Filter by kind (port|shell)",
	)

	return cmd
}

func runList(ctx context.Context, cli labcli.CLI, playID string, opts *listOptions) error {
	_, err := cli.Client().GetPlay(ctx, playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	printer := newListPrinter(cli.OutputStream())
	defer printer.flush()

	printer.printHeader()

	if opts.kind == "" || opts.kind == "port" {
		ports, err := cli.Client().ListPorts(ctx, playID)
		if err != nil {
			return fmt.Errorf("couldn't list ports: %w", err)
		}

		for _, port := range ports {
			printer.printPort(port)
		}
	}

	if opts.kind == "" || opts.kind == "shell" {
		shells, err := cli.Client().ListShells(ctx, playID)
		if err != nil {
			return fmt.Errorf("couldn't list shells: %w", err)
		}

		for _, shell := range shells {
			printer.printShell(shell)
		}
	}

	return nil
}

type listPrinter struct {
	header []string
	writer *tabwriter.Writer
}

func newListPrinter(outStream io.Writer) *listPrinter {
	header := []string{
		"ID",
		"TARGET",
		"URL",
		"ACCESS",
		"HOST REWRITE",
		"PATH REWRITE",
	}

	return &listPrinter{
		header: header,
		writer: tabwriter.NewWriter(outStream, 0, 4, 2, ' ', 0),
	}
}

func (p *listPrinter) printHeader() {
	fmt.Fprintln(p.writer, strings.Join(p.header, "\t"))
}

func (p *listPrinter) printPort(port *api.Port) {
	protocol := "http"
	if port.TLS {
		protocol = "https"
	}

	fields := []string{
		port.ID,
		fmt.Sprintf("%s://%s:%d", protocol, port.Machine, port.Number),
		port.URL,
		string(port.AccessMode),
		port.HostRewrite,
		port.PathRewrite,
	}

	fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
}

func (p *listPrinter) printShell(shell *api.Shell) {
	fields := []string{
		shell.ID,
		fmt.Sprintf("%s@%s", shell.User, shell.Machine),
		shell.URL,
		string(shell.AccessMode),
		"-",
		"-",
	}

	fmt.Fprintln(p.writer, strings.Join(fields, "\t"))
}

func (p *listPrinter) flush() {
	p.writer.Flush()
}
