package playground

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/completion"
	"github.com/iximiuz/labctl/internal/labcli"
)

func newMachineCommand(cli labcli.CLI) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "machine <reboot|stop|restart|console|journal> <playground-id> <machine> [...]",
		Short: "Operate on a single machine of a playground session",
	}

	cmd.AddCommand(
		newMachineRebootCommand(cli),
		newMachineStopCommand(cli),
		newMachineRestartCommand(cli),
		newMachineConsoleCommand(cli),
		newMachineJournalCommand(cli),
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
		Short: "Restart a previously stopped machine of a running playground session",
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
		Short: "Print all serial console files of a machine (one per boot)",
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

type machineJournalOptions struct {
	unit   string
	lines  int
	since  string
	until  string
	cursor string
}

func newMachineJournalCommand(cli labcli.CLI) *cobra.Command {
	var opts machineJournalOptions

	cmd := &cobra.Command{
		Use:   "journal <playground-id> <machine>",
		Short: "Stream a machine's systemd journal (journalctl --follow)",
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
			return labcli.WrapStatusError(runMachineJournal(cmd.Context(), cli, args[0], args[1], &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.unit, "unit", "u", "", "Systemd unit to follow (default: the whole journal)")
	flags.IntVarP(&opts.lines, "lines", "n", 0, "Number of past journal lines to show before following (0 = server default)")
	flags.StringVar(&opts.since, "since", "", "Show entries not older than the given time (e.g. -1h, \"2021-01-01 12:00\")")
	flags.StringVar(&opts.until, "until", "", "Show entries not newer than the given time")
	flags.StringVar(&opts.cursor, "cursor", "", "Show entries after the given journal cursor")

	return cmd
}

func runMachineJournal(ctx context.Context, cli labcli.CLI, playID, machine string, opts *machineJournalOptions) error {
	handle, err := cli.Client().RequestPlayJournal(ctx, playID, api.PlayJournalRequest{
		Machine: machine,
		Unit:    opts.unit,
		Lines:   opts.lines,
		Since:   opts.since,
		Until:   opts.until,
		Cursor:  opts.cursor,
	})
	if err != nil {
		return fmt.Errorf("couldn't start the journal stream: %w", err)
	}

	if opts.unit != "" {
		cli.PrintAux("Streaming journal for unit %q on machine %s (Ctrl-C to stop)...\n", opts.unit, machine)
	} else {
		cli.PrintAux("Streaming journal on machine %s (Ctrl-C to stop)...\n", machine)
	}

	if err := cli.Client().StreamPlayJournal(
		ctx,
		handle.URL,
		cli.Config().WebSocketOrigin(),
		cli.OutputStream(),
		cli.ErrorStream(),
	); err != nil {
		return fmt.Errorf("journal stream failed: %w", err)
	}

	return nil
}
