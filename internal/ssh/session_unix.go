//go:build !windows

package ssh

import (
	"context"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/docker/cli/cli/streams"
	"golang.org/x/crypto/ssh"
)

// dialAgent connects to the SSH agent listening on $SSH_AUTH_SOCK.
// It returns nil (and no error) if no agent is configured.
func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if sock == "" {
		return nil, nil
	}
	return net.Dial("unix", sock)
}

func watchWindowSize(ctx context.Context, out *streams.Out, sess *ssh.Session) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)
	defer signal.Stop(sigCh)

	for {
		select {
		case <-sigCh:
		case <-ctx.Done():
			return nil
		}

		height, width := out.GetTtySize()
		if height > 0 && width > 0 {
			if err := sess.WindowChange(int(height), int(width)); err != nil {
				return err
			}
		}
	}
}
