package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

func newMachineCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machine <reboot|stop|restart|console> <playground-id> <machine> [...]",
		Short: "Operate on a single machine of a playground session",
	}

	cmd.AddCommand(
		newMachineRebootCommand(cli),
		newMachineStopCommand(cli),
		newMachineRestartCommand(cli),
		newMachineConsoleCommand(cli),
	)

	return cmd
}

func newMachineRebootCommand(cli labcli.CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "reboot <playground-id> <machine>",
		Short: "Reboot a machine of a running playground session",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completion.ActivePlays(cli)(cmd, args, toComplete)
			case 1:
				return completion.PlayMachineNames(cli)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runRebootMachine(cmd.Context(), cli, args[0], args[1]))
		},
	}
}

func runRebootMachine(ctx context.Context, cli labcli.CLI, playID, machine string) error {
	cli.PrintAux("Rebooting machine %s of playground %s...\n", machine, playID)

	if _, err := cli.Client().RebootPlayMachine(ctx, playID, machine); err != nil {
		return fmt.Errorf("couldn't reboot the machine: %w", err)
	}

	cli.PrintAux("Machine reboot requested.\n")
	return nil
}

func newMachineStopCommand(cli labcli.CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "stop <playground-id> <machine>",
		Short: "Stop a machine of a running playground session",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completion.ActivePlays(cli)(cmd, args, toComplete)
			case 1:
				return completion.PlayMachineNames(cli)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runStopMachine(cmd.Context(), cli, args[0], args[1]))
		},
	}
}

func runStopMachine(ctx context.Context, cli labcli.CLI, playID, machine string) error {
	cli.PrintAux("Stopping machine %s of playground %s...\n", machine, playID)

	if _, err := cli.Client().StopPlayMachine(ctx, playID, machine); err != nil {
		return fmt.Errorf("couldn't stop the machine: %w", err)
	}

	cli.PrintAux("Machine stop requested.\n")
	return nil
}

func newMachineRestartCommand(cli labcli.CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "restart <playground-id> <machine>",
		Short: "Restart a machine of a running playground session",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completion.ActivePlays(cli)(cmd, args, toComplete)
			case 1:
				return completion.PlayMachineNames(cli)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runRestartMachine(cmd.Context(), cli, args[0], args[1]))
		},
	}
}

func runRestartMachine(ctx context.Context, cli labcli.CLI, playID, machine string) error {
	cli.PrintAux("Restarting machine %s of playground %s...\n", machine, playID)

	if _, err := cli.Client().RestartPlayMachine(ctx, playID, machine); err != nil {
		return fmt.Errorf("couldn't restart the machine: %w", err)
	}

	cli.PrintAux("Machine restart requested.\n")
	return nil
}

func newMachineConsoleCommand(cli labcli.CLI) *cobra.Command {
	return &cobra.Command{
		Use:   "console <playground-id> <machine>",
		Short: "Print all serial console files of a machine, separated by clear headers",
		Args:  cobra.ExactArgs(2),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			switch len(args) {
			case 0:
				return completion.NonDestroyedPlays(cli)(cmd, args, toComplete)
			case 1:
				return completion.PlayMachineNames(cli)(cmd, args, toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return labcli.WrapStatusError(runMachineConsole(cmd.Context(), cli, args[0], args[1]))
		},
	}
}

func runMachineConsole(ctx context.Context, cli labcli.CLI, playID, machine string) error {
	consoles, err := cli.Client().ListPlayMachineConsoles(ctx, playID, machine)
	if err != nil {
		return fmt.Errorf("couldn't list machine consoles: %w", err)
	}

	for i, name := range consoles {
		if i > 0 {
			cli.PrintOut("\n")
		}
		cli.PrintOut("===== %s =====\n", name)

		content, err := cli.Client().ReadPlayMachineConsole(ctx, playID, machine, name)
		if err != nil {
			return fmt.Errorf("couldn't read machine console %s: %w", name, err)
		}

		cli.PrintOut("%s", content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			cli.PrintOut("\n")
		}
	}
	return nil
}
