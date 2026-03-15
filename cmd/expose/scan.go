package expose

import (
	"context"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

type scanOptions struct {
	machine string
}

func NewScanCommand(cli labcli.CLI) *cobra.Command {
	var opts scanOptions

	cmd := &cobra.Command{
		Use:               "scan <playground>",
		Short:             "Scan for exposable HTTP(S) ports in a running playground (best effort)",
		Args:              cobra.ExactArgs(1),
		ValidArgsFunction: completion.ActivePlays(cli),
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runScan(cmd.Context(), cli, args[0], &opts))
		},
	}

	flags := cmd.Flags()
	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		"Scan only ports on a specific machine",
	)

	return cmd
}

func runScan(ctx context.Context, cli labcli.CLI, playID string, opts *scanOptions) error {
	if _, err := cli.Client().GetPlay(ctx, playID); err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	cli.PrintAux("Scanning for open HTTP(S) ports...\n")

	ports, err := cli.Client().ScanPorts(ctx, playID, opts.machine)
	if err != nil {
		return fmt.Errorf("couldn't scan ports: %w", err)
	}

	httpPorts := filterHTTPPorts(ports)

	if len(httpPorts) == 0 {
		cli.PrintAux("No open HTTP(S) ports detected.\n")
		return nil
	}

	printer := newScanPrinter(cli.OutputStream())
	defer printer.flush()

	printer.printHeader()
	for _, p := range httpPorts {
		printer.printPort(p)
	}

	return nil
}

// filterHTTPPorts returns only HTTP/HTTPS ports, deduplicated by (machine, number).
// When the same port is detected on multiple interfaces, the first occurrence wins
// (preferring HTTPS over HTTP if both are present since scan results typically list
// HTTPS first).
func filterHTTPPorts(ports []*api.ScannedPort) []*api.ScannedPort {
	type key struct {
		machine string
		number  int
	}

	seen := map[key]struct{}{}
	var result []*api.ScannedPort

	for _, p := range ports {
		if p.Protocol != "HTTP" && p.Protocol != "HTTPS" {
			continue
		}

		k := key{p.Machine, p.Number}
		if _, ok := seen[k]; ok {
			continue
		}

		seen[k] = struct{}{}
		result = append(result, p)
	}

	return result
}

type scanPrinter struct {
	writer *tabwriter.Writer
}

func newScanPrinter(outStream io.Writer) *scanPrinter {
	return &scanPrinter{
		writer: tabwriter.NewWriter(outStream, 0, 4, 2, ' ', 0),
	}
}

func (p *scanPrinter) printHeader() {
	fmt.Fprintln(p.writer, strings.Join([]string{"MACHINE", "PORT", "PROTOCOL"}, "\t"))
}

func (p *scanPrinter) printPort(port *api.ScannedPort) {
	fmt.Fprintln(p.writer, strings.Join([]string{
		port.Machine,
		fmt.Sprintf("%d", port.Number),
		port.Protocol,
	}, "\t"))
}

func (p *scanPrinter) flush() {
	p.writer.Flush()
}
