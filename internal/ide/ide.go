// Package ide centralizes the logic for opening labctl playgrounds in local
// IDEs (VSCode-family editors and Zed) over an SSH proxy.
package ide

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"slices"
	"strings"
)

const (
	Antigravity = "antigravity"
	VSCode      = "code"
	Cursor      = "cursor"
	Windsurf    = "windsurf"
	Zed         = "zed"
)

// Supported lists the IDE CLI names labctl knows how to open, in display order.
var Supported = []string{Antigravity, VSCode, Cursor, Windsurf, Zed}

// IsSupported reports whether name is one of the supported IDEs.
func IsSupported(name string) bool {
	return slices.Contains(Supported, name)
}

// SupportedList returns the supported IDE names as a quoted, comma-separated
// string for help text and error messages.
func SupportedList() string {
	quoted := make([]string, len(Supported))
	for i, s := range Supported {
		quoted[i] = fmt.Sprintf("%q", s)
	}
	return strings.Join(quoted, ", ")
}

// EnsureInstalled returns a friendly error if the IDE's CLI binary isn't on PATH.
func EnsureInstalled(ide string) error {
	if _, err := exec.LookPath(ide); err != nil {
		return fmt.Errorf("couldn't find the %q binary in PATH - is %s installed and its CLI command available?", ide, ide)
	}
	return nil
}

// LaunchArgs returns the arguments to pass to the IDE binary to open workDir on
// the remote machine reachable at host:port as user over the SSH proxy.
func LaunchArgs(ide, user, host, port, workDir string) []string {
	if ide == Zed {
		// Zed opens a remote folder via a positional ssh:// URI.
		return []string{fmt.Sprintf("ssh://%s@%s:%s%s", user, host, port, workDir)}
	}
	// VSCode-family editors use the --folder-uri flag.
	return []string{
		"--folder-uri",
		fmt.Sprintf("vscode-remote://ssh-remote+%s@%s:%s%s", user, host, port, workDir),
	}
}

// Command builds the exec.Cmd that launches the IDE with args, taking care of
// the extra cmd /C wrapping required on Windows.
func Command(ctx context.Context, ide string, args []string) *exec.Cmd {
	if runtime.GOOS == "windows" {
		return exec.CommandContext(ctx, "cmd", append([]string{"/C", ide}, args...)...)
	}
	return exec.CommandContext(ctx, ide, args...)
}

// UserHomeDir returns the remote home directory for the given login user.
func UserHomeDir(user string) string {
	if user == "root" {
		return "/root"
	}
	return fmt.Sprintf("/home/%s", user)
}
