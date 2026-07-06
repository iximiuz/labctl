package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/completion"
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

	forwardAgent bool
}

func NewCommand(cli labcli.CLI) *cobra.Command {
	var opts options

	cmd := &cobra.Command{
		Use:               "ssh [flags] <playground-id>",
		Short:             `Start SSH session to the target playground`,
		Example:           example,
		Args:              cobra.MinimumNArgs(1),
		ValidArgsFunction: completion.ActivePlays(cli),
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
	flags.BoolVar(
		&opts.forwardAgent,
		"forward-agent",
		false,
		`INSECURE: Forward the SSH agent to the playground VM (use at your own risk)`,
	)

	return cmd
}

func runSSHSession(ctx context.Context, cli labcli.CLI, opts *options) error {
	p, err := cli.Client().GetPlay(ctx, opts.playID)
	if err != nil {
		return fmt.Errorf("couldn't get playground: %w", err)
	}

	if opts.machine, err = p.ResolveMachine(opts.machine); err != nil {
		return err
	}
	if opts.user, err = p.ResolveUser(opts.machine, opts.user); err != nil {
		return err
	}

	sess, errCh, err := StartSSHSession(ctx, cli, p, opts.machine, opts.user, opts.command, opts.forwardAgent)
	if err != nil {
		return fmt.Errorf("couldn't start SSH session: %w", err)
	}

	if err := <-errCh; err != nil {
		return err
	}

	if err := sess.Wait(); err != nil {
		slog.Debug("SSH session wait said: " + err.Error())
	}

	return nil
}

func StartSSHSession(
	ctx context.Context,
	cli labcli.CLI,
	play *api.Play,
	machine string,
	user string,
	command []string,
	forwardAgent bool,
) (*ssh.Session, <-chan error, error) {
	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:          play.ID,
		Machine:         machine,
		SSHUser:         user,
		SSHIdentityFile: cli.Config().SSHIdentityFile,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't start tunnel: %w", err)
	}

	localPort := portforward.RandomLocalPort()

	ctx, cancel := context.WithCancel(ctx)

	go func() {
		status := tunnel.StartForwarding(ctx, portforward.ForwardingSpec{
			LocalPort:  localPort,
			RemotePort: "22",
		})
		if err := <-status; err != nil {
			slog.Debug("Tunnel forwarding exited with error", "error", err.Error())
		}
	}()

	// The local listener starts accepting connections before the tunnel is
	// ready end-to-end, so a successful dial doesn't mean the remote sshd is
	// reachable yet - the first SSH handshakes may be dropped with a
	// connection reset. Retry the dial and the handshake together, with a
	// fresh connection per attempt, bailing out early on permanent errors
	// (e.g. authentication failures).
	var (
		dial net.Dialer
		conn net.Conn
		sess *ssh.Session
		addr = "localhost:" + localPort
	)
	if err := retry.UntilSuccess(ctx, func() error {
		conn, err = dial.DialContext(ctx, "tcp", addr)
		if err != nil {
			return fmt.Errorf("couldn't connect to the forwarded SSH port %s: %w", addr, err)
		}

		sess, err = ssh.NewSession(conn, user, cli.Config().SSHIdentityFile, forwardAgent)
		if err != nil {
			conn.Close()
			err = fmt.Errorf("couldn't create SSH session: %w", err)
			if isTransientSSHError(err) {
				return err
			}
			return retry.Unrecoverable(err)
		}

		return nil
	}, 60, 1*time.Second); err != nil {
		cancel()
		return nil, nil, err
	}

	runErrCh := make(chan error, 1)

	go func() {
		defer conn.Close()
		defer cancel()
		defer close(runErrCh)

		err := sess.Run(ctx, cli, strings.Join(command, " "))
		if err != nil {
			runErrCh <- err
		}
	}()

	return sess, runErrCh, nil
}

// isTransientSSHError tells apart transport failures that are likely to go
// away on retry (the tunnel isn't ready end-to-end yet, or the connection
// was dropped mid-handshake) from permanent ones, such as authentication
// failures, that will only fail again.
func isTransientSSHError(err error) bool {
	if strings.Contains(err.Error(), "unable to authenticate") {
		return false
	}

	var netErr net.Error
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, net.ErrClosed) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, syscall.ECONNREFUSED) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.As(err, &netErr)
}
