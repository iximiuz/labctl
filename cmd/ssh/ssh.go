package ssh

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/retry"
	"github.com/iximiuz/labctl/internal/ssh"
)

const example = `  # SSH into the first machine in the playground
  labctl ssh 65e78a64366c2b0cf9ddc34c

  # SSH into the machine named "node-02"
	labctl ssh 65e78a64366c2b0cf9ddc34c --machine node-02

	# Execute a command on the remote machine
	labctl ssh 65e78a64366c2b0cf9ddc34c -- ls -la /`

type options struct {
	playID  string
	machine string

	command []string
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:     "ssh [flags] <playground-id>",
		Short:   `Start SSH session to the target playground`,
		Example: example,
		Args:    cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.playID = args[0]
			opts.command = cmd.Flags().Args()[1:]

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

func runSSHSession(ctx context.Context, cli labcli.CLI, opts *options) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine == "" {
		opts.machine = p.Machines[0].Name
	} else {
		if p.GetMachine(opts.machine) == nil {
			return fmt.Errorf("machine %q not found in the playground", opts.machine)
		}
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:   opts.playID,
		Machine:  opts.machine,
		PlaysDir: cli.Config().PlaysDir,
		SSHDir:   cli.Config().SSHDir,
	})
	if err != nil {
		return fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		localPort = portforward.RandomLocalPort()
		errCh     = make(chan error, 100)
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			RemotePort: "22",
		}, errCh); err != nil {
			errCh <- err
		}
		close(errCh)
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

	var (
		dial net.Dialer
		conn net.Conn
		addr = "localhost:" + localPort
	)
	if err := retry.UntilSuccess(ctx, func() error {
		conn, err = dial.DialContext(ctx, "tcp", addr)
		return err
	}, 10, 1*time.Second); err != nil {
		return fmt.Errorf("couldn't connect to the forwarded SSH port %s: %w", addr, err)
	}
	defer conn.Close()

	sess, err := ssh.NewSession(conn, "root", cli.Config().SSHDir)
	if err != nil {
		return fmt.Errorf("couldn't create SSH session: %w", err)
	}

	if err := sess.Run(ctx, cli, strings.Join(opts.command, " ")); err != nil {
		return fmt.Errorf("SSH session error: %w", err)
	}

	return nil
}
