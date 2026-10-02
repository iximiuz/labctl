//go:build windows

package ssh

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh/agent"
)

// An unusable $SSH_AUTH_SOCK (e.g. an MSYS agent socket from Git Bash) falls
// back to the Windows OpenSSH agent.
func TestDialAgent_FallsBackFromBrokenSocket(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")
	if conn, err := dialAgent(); err != nil || conn == nil {
		t.Skip("the Windows OpenSSH agent is not running")
	} else {
		conn.Close()
	}

	t.Setenv("SSH_AUTH_SOCK", "/tmp/ssh-XXXXXX/agent.1234")

	conn, err := dialAgent()
	require.NoError(t, err)
	require.NotNil(t, conn)
	defer conn.Close()

	_, err = agent.NewClient(conn).List()
	require.NoError(t, err)
}

// Regression test: a synchronously opened agent pipe hangs on the second
// request. Needs the Windows OpenSSH agent (ssh-agent service) running.
func TestDialAgent_RepeatedRequests(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", "")

	conn, err := dialAgent()
	require.NoError(t, err)
	if conn == nil {
		t.Skip("the Windows OpenSSH agent is not running")
	}
	defer conn.Close()

	client := agent.NewClient(conn)

	done := make(chan error, 1)
	go func() {
		for i := 0; i < 3; i++ {
			if _, err := client.List(); err != nil {
				done <- err
				return
			}
		}
		done <- nil
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("SSH agent requests hung")
	}
}
