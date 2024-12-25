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
	user    string

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
	flags.StringVarP(
		&opts.user,
		"user",
		"u",
		"",
		`SSH user (default: the machine's default login user)`,
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

	if opts.user == "" {
		if u := p.GetMachine(opts.machine).DefaultUser(); u != nil {
			opts.user = u.Name
		} else {
			opts.user = "root"
		}
	}
	if !p.GetMachine(opts.machine).HasUser(opts.user) {
		return fmt.Errorf("user %q not found in the machine %q", opts.user, opts.machine)
	}

	if sess, err := StartSSHSession(ctx, cli, opts.playID, opts.machine, opts.user, opts.command); err != nil {
		return fmt.Errorf("couldn't start SSH session: %w", err)
	} else {
		if err := sess.Wait(); err != nil {
			slog.Debug("SSH session wait said: " + err.Error())
		}
	}

	return nil
}

func StartSSHSession(
	ctx context.Context,
	cli labcli.CLI,
	playID string,
	machine string,
	user string,
	command []string,
) (*ssh.Session, error) {
	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:   playID,
		Machine:  machine,
		PlaysDir: cli.Config().PlaysDir,
		SSHUser:  user,
		SSHDir:   cli.Config().SSHDir,
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't start tunnel: %w", err)
	}

	var (
		localPort = portforward.RandomLocalPort()
		errCh     = make(chan error, 100)
	)
	defer close(errCh)

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		if err := tunnel.Forward(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
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
				if err != nil {
					slog.Debug("Tunnel borked", "error", err.Error())
				}
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
	}, 60, 1*time.Second); err != nil {
		cancel()
		return nil, fmt.Errorf("couldn't connect to the forwarded SSH port %s: %w", addr, err)
	}

	sess, err := ssh.NewSession(conn, user, cli.Config().SSHDir)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("couldn't create SSH session: %w", err)
	}

	go func() {
		defer conn.Close()
		defer cancel()
		if err := sess.Run(ctx, cli, strings.Join(command, " ")); err != nil {
			slog.Error("SSH session error", "error", err.Error())
		}
	}()

	return sess, nil
}
