//go:build !windows

package ssh

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/crypto/ssh"
	"golang.org/x/term"
)

const (
	DefaultHeight = 40
	DefaultWidth  = 80
)

var modes = ssh.TerminalModes{
	ssh.ECHO:          0,     // disable echoing
	ssh.TTY_OP_ISPEED: 14400, // input speed = 14.4kbaud
	ssh.TTY_OP_OSPEED: 14400, // output speed = 14.4kbaud
}

type Client struct {
	addr string
	user string

	sshKeyPath string

	client *ssh.Client
	conn   ssh.Conn
}

func NewClient(addr, user, sshKeyPath string) *Client {
	return &Client{
		addr:       addr,
		user:       user,
		sshKeyPath: sshKeyPath,
	}
}

type connResp struct {
	err    error
	conn   ssh.Conn
	client *ssh.Client
}

func (c *Client) Connect(ctx context.Context) error {
	privateKey, err := ReadPrivateKey(c.sshKeyPath)
	if err != nil {
		return err
	}

	keySigner, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return err
	}

	var d net.Dialer
	tcpConn, err := d.DialContext(ctx, "tcp", c.addr)
	if err != nil {
		return err
	}

	conf := &ssh.ClientConfig{
		User: c.user,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(keySigner),
		},
		HostKeyCallback:   ssh.InsecureIgnoreHostKey(),
		HostKeyAlgorithms: []string{ssh.KeyAlgoED25519},
	}

	respCh := make(chan connResp)

	// ssh.NewClientConn doesn't take a context, so we need to handle cancelation on our end
	go func() {
		conn, chans, reqs, err := ssh.NewClientConn(tcpConn, tcpConn.RemoteAddr().String(), conf)
		if err != nil {
			respCh <- connResp{err: err}
			return
		}

		client := ssh.NewClient(conn, chans, reqs)

		respCh <- connResp{nil, conn, client}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case resp := <-respCh:
			if resp.err != nil {
				return resp.err
			}
			c.conn = resp.conn
			c.client = resp.client
			return nil
		}
	}
}

func (c *Client) Shell(ctx context.Context, sessIO *SessionIO, cmd string) error {
	if c.client == nil {
		if err := c.Connect(ctx); err != nil {
			return err
		}
	}

	sess, err := c.client.NewSession()
	if err != nil {
		return err
	}
	defer sess.Close()

	return sessIO.attach(ctx, sess, cmd)
}

func (c *Client) Close() error {
	if c.conn != nil {
		if err := c.conn.Close(); err != nil {
			return err
		}
	}

	c.conn = nil
	return nil
}

type SessionIO struct {
	Stdin  io.Reader
	Stdout io.WriteCloser
	Stderr io.WriteCloser

	AllocPTY bool
	TermEnv  string
}

func (s *SessionIO) attach(ctx context.Context, sess *ssh.Session, cmd string) error {
	if s.AllocPTY {
		width, height := DefaultWidth, DefaultHeight

		if fd, ok := getFd(s.Stdin); ok {
			state, err := term.MakeRaw(fd)
			if err != nil {
				return err
			}
			defer term.Restore(fd, state)
		}

		if w, h, err := s.getAndWatchSize(ctx, sess); err == nil {
			width, height = w, h
		}

		if err := sess.RequestPty(s.TermEnv, height, width, modes); err != nil {
			return err
		}
	}

	var closeStdin sync.Once
	stdin, err := sess.StdinPipe()
	if err != nil {
		return err
	}
	defer closeStdin.Do(func() {
		stdin.Close()
	})

	stdout, err := sess.StdoutPipe()
	if err != nil {
		return err
	}

	stderr, err := sess.StderrPipe()
	if err != nil {
		return err
	}

	go func() {
		defer closeStdin.Do(func() {
			stdin.Close()
		})
		if s.Stdin != nil {
			io.Copy(stdin, s.Stdin)
		}
	}()
	if s.Stdout != nil {
		go io.Copy(s.Stdout, stdout)
	}

	if s.Stderr != nil {
		go io.Copy(s.Stderr, stderr)
	}

	cmdC := make(chan error, 1)
	go func() {
		defer close(cmdC)

		if cmd == "" {
			err = sess.Shell()
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

func (s *SessionIO) getAndWatchSize(ctx context.Context, sess *ssh.Session) (int, int, error) {
	fd, ok := getFd(s.Stdin)
	if !ok {
		return 0, 0, errors.New("could not get console handle")
	}

	width, height, err := term.GetSize(fd)
	if err != nil {
		return 0, 0, err
	}

	go func() {
		if err := watchWindowSize(ctx, fd, sess); err != nil {
			slog.Debug("Error watching window size", err)
		}
	}()

	return width, height, nil
}

func watchWindowSize(ctx context.Context, fd int, sess *ssh.Session) error {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, syscall.SIGWINCH)

	for {
		select {
		case <-sigc:
		case <-ctx.Done():
			return nil
		}

		width, height, err := term.GetSize(fd)
		if err != nil {
			return err
		}

		if err := sess.WindowChange(height, width); err != nil {
			return err
		}
	}
}

// FdReader is an io.Reader with an Fd function
type FdReader interface {
	io.Reader
	Fd() uintptr
}

func getFd(reader io.Reader) (fd int, ok bool) {
	fdthing, ok := reader.(FdReader)
	if !ok {
		return 0, false
	}

	fd = int(fdthing.Fd())
	return fd, term.IsTerminal(fd)
}
