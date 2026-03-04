package expose

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"time"

	"github.com/iximiuz/labctl/api"
	"github.com/iximiuz/labctl/internal/labcli"
	"github.com/iximiuz/labctl/internal/portforward"
	"github.com/iximiuz/labctl/internal/retry"
	"github.com/iximiuz/labctl/internal/ssh"
)

type discoveredPort struct {
	Port      int
	Service   string
	Namespace string
}

func (d discoveredPort) Label() string {
	if d.Service != "" {
		return fmt.Sprintf("%s/%s (NodePort %d)", d.Namespace, d.Service, d.Port)
	}
	return strconv.Itoa(d.Port)
}

// connectSSH establishes a non-interactive SSH session to a playground machine
// by starting a WebSocket tunnel, forwarding port 22, and connecting via SSH.
// The caller must call the returned cancel function when done.
func connectSSH(
	ctx context.Context,
	cli labcli.CLI,
	play *api.Play,
	machine string,
) (*ssh.Session, context.CancelFunc, error) {
	user := "root"
	if m := play.GetMachine(machine); m != nil {
		if u := m.DefaultUser(); u != nil {
			user = u.Name
		}
	}

	tunnel, err := portforward.StartTunnel(ctx, cli.Client(), portforward.TunnelOptions{
		PlayID:          play.ID,
		Machine:         machine,
		SSHUser:         user,
		SSHIdentityFile: cli.Config().SSHIdentityFile,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("couldn't start tunnel to %s: %w", machine, err)
	}

	localPort := portforward.RandomLocalPort()
	errCh := make(chan error, 100)

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
					slog.Debug("Tunnel error", "machine", machine, "error", err.Error())
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
		return nil, nil, fmt.Errorf("couldn't connect to SSH on %s (%s): %w", machine, addr, err)
	}

	sess, err := ssh.NewSession(conn, user, cli.Config().SSHIdentityFile, false)
	if err != nil {
		cancel()
		conn.Close()
		return nil, nil, fmt.Errorf("couldn't create SSH session to %s: %w", machine, err)
	}

	return sess, cancel, nil
}
