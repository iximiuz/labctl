//go:build !windows

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/docker/cli/cli/streams"
	"github.com/iximiuz/labctl/internal/labcli"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const defaultTermEnv = "xterm-256color"

type Session struct {
	client       *ssh.Client
	forwardAgent bool
}

func NewSession(
	conn net.Conn,
	user string,
	sshKeyPath string,
	forwardAgent bool,
) (*Session, error) {
	var authMethods []ssh.AuthMethod

	// Try SSH agent first
	var sshAgent agent.Agent
	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		agentConn, err := net.Dial("unix", sock)
		if err != nil {
			slog.Debug("Failed to connect to SSH agent", "error", err)
		} else {
			sshAgent = agent.NewClient(agentConn)
			signers, err := sshAgent.Signers()
			if err != nil {
				slog.Debug("Failed to retrieve signers from SSH agent", "error", err)
			} else if len(signers) > 0 {
				authMethods = append(authMethods, ssh.PublicKeys(signers...))
			}
		}
	}

	privateKey, err := readPrivateKey(sshKeyPath)
	if err != nil {
		slog.Debug("Failed to read SSH private key", "error", err)
	} else {
		keySigner, err := ssh.ParsePrivateKey([]byte(privateKey))
		if err != nil {
			slog.Debug("Failed to parse SSH private key", "error", err)
		} else {
			// The key (if exists) takes precedence over the agent.
			authMethods = append([]ssh.AuthMethod{ssh.PublicKeys(keySigner)}, authMethods...)
		}
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

	forwardAgent = forwardAgent && sshAgent != nil
	if forwardAgent {
		agent.ForwardToAgent(client, sshAgent)
	}

	return &Session{
		client:       client,
		forwardAgent: forwardAgent,
	}, nil
}

func (s *Session) Run(ctx context.Context, streams labcli.Streams, cmd string) error {
	sess, err := s.client.NewSession()
	if err != nil {
		return fmt.Errorf("create SSH session: %w", err)
	}
	defer sess.Close()

	if s.forwardAgent {
		if err := agent.RequestAgentForwarding(sess); err != nil {
			slog.Warn("Failed to forward SSH agent", "error", err.Error())
		}
	}

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
				// ssh.ECHO:          0,  // disable echoing
				ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
				ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
			}); err != nil {
				return fmt.Errorf("request PTY: %w", err)
			}

			go func() {
				if err := watchWindowSize(ctx, streams.OutputStream(), sess); err != nil {
					slog.Debug("Error watching window size", "error", err.Error())
				}
			}()
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

func watchWindowSize(ctx context.Context, out *streams.Out, sess *ssh.Session) error {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGWINCH)

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
