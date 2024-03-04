package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/ssh"
)

type sshOptions struct {
	playID  string
	machine string
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts sshOptions

	cmd := &cobra.Command{
		Use:   "ssh [flags] <playground-id>",
		Short: `Start SSH session to the target playground`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]

			return labcli.WrapStatusError(runSSHSession(cmd.Context(), cli, &opts))
		},
	}

	flags := cmd.Flags()

	flags.StringVarP(
		&opts.machine,
		"machine",
		"m",
		"",
		`Target machine (default: the first machine in the playground)`,
	)

	return cmd
}

func runSSHSession(ctx context.Context, cli labcli.CLI, opts *sshOptions) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine == "" {
		opts.machine = p.Machines[0].Name
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:     opts.playID,
		Machine:    opts.machine,
		SSHDirPath: cli.Config().SSHDirPath,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		localPort = 40000 + rand.Intn(20000)
		errCh     = make(chan error, 100)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  fmt.Sprintf("%d", localPort),
			RemotePort: "22",
		}, errCh); err != nil {
			errCh <- err
		}
	}()

	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case err := <-errCh:
				slog.Debug("Tunnel error: %v", err)
			}
		}
	}()

	time.Sleep(2 * time.Second)

	client := ssh.NewClient(fmt.Sprintf("localhost:%d", localPort), "root", cli.Config().SSHDirPath)
	if err := client.Shell(ctx, &ssh.SessionIO{
		Stdin:    cli.InputStream(),
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		AllocPTY: true,
	}, "bash"); err != nil {
		return fmt.Errorf("couldn't start SSH session: %w", err)
	}

	return nil
}
