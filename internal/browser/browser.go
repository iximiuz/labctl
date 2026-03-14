package browser

import (
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/skratchdot/open-golang/open"
)

// Printer is a minimal output interface, decoupled from labcli to avoid
// heavy transitive dependencies (docker/cli, charmbracelet/huh).
type Printer interface {
	PrintAux(string, ...any)
}

var isWSL = sync.OnceValue(func() bool {
	// Primary: /proc/version contains "microsoft" on both WSL1 and WSL2.
	data, err := os.ReadFile("/proc/version")
	if err == nil && detectWSL(string(data)) {
		return true
	}

	// Secondary: WSLInterop file exists.
	_, err = os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop")
	return err == nil
})

func detectWSL(procVersion string) bool {
	lower := strings.ToLower(procVersion)
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

// Open opens the given URL in the user's default browser.
// On WSL, it tries wslview and cmd.exe before falling back to xdg-open.
func Open(url string) error {
	if isWSL() {
		return openWSL(url)
	}
	return open.Run(url)
}

// tryExec looks up a command by name and runs it with the given args.
// Returns true if the command was found and completed successfully (exit 0).
func tryExec(name string, args ...string) bool {
	path, err := exec.LookPath(name)
	if err != nil {
		return false
	}
	return exec.Command(path, args...).Run() == nil
}

func openWSL(url string) error {
	if tryExec("wslview", url) {
		return nil
	}

	// The empty "" arg prevents cmd.exe from interpreting the URL as a window title.
	if tryExec("cmd.exe", "/c", "start", "", `"`+url+`"`) {
		return nil
	}

	return open.Run(url)
}

// openFunc is the function used by OpenWithFallbackMessage to open URLs.
// It defaults to Open and can be overridden in tests to avoid spawning
// real browser processes.
var openFunc = Open

// OpenWithFallbackMessage opens the URL and prints a prominent, copy-friendly
// fallback message if the browser cannot be opened.
func OpenWithFallbackMessage(out Printer, url string) {
	out.PrintAux("Opening %s in your browser...\n", url)

	if err := openFunc(url); err != nil {
		out.PrintAux("\nCould not open browser automatically.\n")
		out.PrintAux("Please open this URL in your browser:\n\n")
		out.PrintAux("  %s\n\n", url)
	}
}
