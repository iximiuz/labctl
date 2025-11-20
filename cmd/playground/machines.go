package playground

import (
	"context"
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
)

func newMachinesCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machines <playground-id>",
		Short: `List machines of a specific playground session`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			playgroundID := args[0]
			return labcli.WrapStatusError(runListMachines(cmd.Context(), cli, playgroundID))
		},
	}

	return cmd
}

func runListMachines(ctx context.Context, cli labcli.CLI, playgroundID string) error {
	play, err := cli.Client().GetPlay(ctx, playgroundID)
	if err != nil {
		return fmt.Errorf("couldn't find playground with ID %s: %w", playgroundID, err)
	}

	writer := tabwriter.NewWriter(cli.OutputStream(), 0, 4, 2, ' ', 0)
	defer writer.Flush()

	fmt.Fprintln(writer, "MACHINE NAME")

	for _, machine := range play.Machines {
		fmt.Fprintln(writer, machine.Name)
	}

	return nil
}
