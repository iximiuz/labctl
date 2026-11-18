//go:build windows

package ssh

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net"
	"os"
	"strings"
	"time"

	"github.com/docker/cli/cli/streams"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/windows"
)

// The named pipe of the Windows built-in OpenSSH agent (the ssh-agent service).
const defaultAgentPipe = `\\.\pipe\openssh-ssh-agent`

// dialAgent connects to the SSH agent: a named pipe or an AF_UNIX socket from
// $SSH_AUTH_SOCK, or the Windows OpenSSH agent's pipe by default. It returns
// nil (and no error) if the default agent isn't running.
func dialAgent() (io.ReadWriteCloser, error) {
	sock := os.Getenv("SSH_AUTH_SOCK")
	if isNamedPipe(sock) {
		f, err := openPipe(sock)
		if err != nil {
			return nil, fmt.Errorf("open named pipe %s: %w", sock, err)
		}
		return f, nil
	}

	if sock != "" {
		conn, err := net.Dial("unix", sock)
		if err == nil {
			return conn, nil
		}
		// E.g., an MSYS/Cygwin agent socket from Git Bash, which isn't a real
		// AF_UNIX socket - fall back to the Windows OpenSSH agent.
		slog.Debug("Failed to connect to SSH agent from SSH_AUTH_SOCK; trying the Windows OpenSSH agent",
			"SSH_AUTH_SOCK", sock, "error", err)
	}

	f, err := openPipe(defaultAgentPipe)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open named pipe %s: %w", defaultAgentPipe, err)
	}
	return f, nil
}

// openPipe opens a named pipe client handle for overlapped (asynchronous)
// I/O, which os.NewFile hands over to the runtime poller. A pipe opened with
// os.OpenFile gets a synchronous handle instead, and the agent client hangs
// on its second request.
func openPipe(path string) (*os.File, error) {
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}

	for attempt := 0; ; attempt++ {
		h, err := windows.CreateFile(
			name,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			0,
			nil,
			windows.OPEN_EXISTING,
			windows.FILE_FLAG_OVERLAPPED|windows.SECURITY_SQOS_PRESENT|windows.SECURITY_ANONYMOUS,
			0,
		)
		if err == nil {
			return os.NewFile(uintptr(h), path), nil
		}

		// All pipe instances are busy serving other clients - retry shortly.
		if errors.Is(err, windows.ERROR_PIPE_BUSY) && attempt < 10 {
			time.Sleep(100 * time.Millisecond)
			continue
		}
		return nil, err
	}
}

func isNamedPipe(path string) bool {
	return strings.HasPrefix(path, `\\.\pipe\`) || strings.HasPrefix(path, `//./pipe/`)
}

// Windows has no SIGWINCH, so the console size is polled instead.
const windowSizePollInterval = 250 * time.Millisecond

func watchWindowSize(ctx context.Context, out *streams.Out, sess *ssh.Session) error {
	lastHeight, lastWidth := out.GetTtySize()

	ticker := time.NewTicker(windowSizePollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return nil
		}

		height, width := out.GetTtySize()
		if height == 0 || width == 0 || (height == lastHeight && width == lastWidth) {
			continue
		}
		lastHeight, lastWidth = height, width

		if err := sess.WindowChange(int(height), int(width)); err != nil {
			return err
		}
	}
}
