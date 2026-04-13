//go:build windows

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"

	"github.com/iximiuz/labctl/internal/labcli"
	"golang.org/x/crypto/ssh"
)

const defaultTermEnv = "xterm-256color"

type Session struct {
	client *ssh.Client
}

func NewSession(
	conn net.Conn,
	user string,
	sshKeyPath string,
	forwardAgent bool,
) (*Session, error) {
	var authMethods []ssh.AuthMethod

	privateKey, err := readPrivateKey(sshKeyPath)
	if err != nil {
		slog.Debug("Failed to read SSH private key", "error", err)
	} else {
		keySigner, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			slog.Debug("Failed to parse SSH private key", "error", err)
		} else {
			authMethods = append(authMethods, ssh.PublicKeys(keySigner))
		}
	}

	if forwardAgent {
		slog.Warn("SSH agent forwarding is not supported on Windows")
	}

	sshConn, chans, reqs, err := ssh.NewClientConn(conn, conn.RemoteAddr().String(), &ssh.ClientConfig{
		User:              user,
		Auth:              authMethods,
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
	})
	if err != nil {
		return nil, fmt.Errorf("create SSH client connection: %w", err)
	}

	client := ssh.NewClient(sshConn, chans, reqs)
	return &Session{client: client}, nil
}

func (s *Session) Run(ctx context.Context, streams labcli.Streams, cmd string) error {
	sess, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer sess.Close()

	if streams.InputStream().IsTerminal() {
		if err := streams.InputStream().SetRawTerminal(); err != nil {
			slog.Warn("Could not enable raw terminal mode", "error", err.Error())
		} else {
			defer streams.InputStream().RestoreTerminal()

			height, width := streams.OutputStream().GetTtySize()
			if height == 0 {
				height = 40
			}
			if width == 0 {
				width = 80
			}

			if err := sess.RequestPty(defaultTermEnv, int(height), int(width), ssh.TerminalModes{
				ssh.TTY_OP_ISPEED: 14400,
				ssh.TTY_OP_OSPEED: 14400,
			}); err != nil {
				return fmt.Errorf("request PTY: %w", err)
			}
		}
	}

	sess.Stdout = streams.OutputStream()
	sess.Stderr = streams.ErrorStream()

	var closeStdin sync.Once
	stdin, err := sess.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin pipe: %w", err)
	}

	go func() {
		defer closeStdin.Do(func() {
			stdin.Close()
		})

		io.Copy(stdin, streams.InputStream())
	}()

	cmdC := make(chan error, 1)
	go func() {
		defer close(cmdC)

		var err error
		if cmd == "" {
			err = sess.Shell()
			if err == nil {
				err = sess.Wait()
			}
		} else {
			err = sess.Run(cmd)
		}

		if err != nil && err != io.EOF {
			cmdC <- err
		}
	}()

	select {
	case err := <-cmdC:
		return err
	case <-ctx.Done():
		return errors.New("session forcibly closed; the remote process may still be running")
	}
}

func (s *Session) Close() error {
	return s.client.Close()
}

func (s *Session) Wait() error {
	return s.client.Wait()
}
