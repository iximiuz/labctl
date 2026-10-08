package content

import (
	"bytes"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// warned keeps watch mode from repeating the same warning on every archive
// rebuild.
var warned sync.Map

func warnOnce(key, msg string, args ...any) {
	if _, ok := warned.LoadOrStore(key, true); !ok {
		slog.Warn(msg, args...)
	}
}

// archiveEntry is a regular file to be archived.
type archiveEntry struct {
	path string // local path
	name string // slash-separated path relative to the archived folder
	info fs.FileInfo
}

// gitCommand runs git in dir, isolated from GIT_* variables that would point
// it at another repository (e.g. when labctl runs from a git hook).
func gitCommand(dir string, args ...string) *exec.Cmd {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	for _, kv := range os.Environ() {
		switch strings.SplitN(kv, "=", 2)[0] {
		case "GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_COMMON_DIR":
			continue
		}
		cmd.Env = append(cmd.Env, kv)
	}
	return cmd
}

// gitIndexModes returns the file modes (e.g. "100644", "100755") recorded in
// the git index for the files under folder, keyed by slash-separated paths
// relative to folder.
func gitIndexModes(folder string) (map[string]string, error) {
	out, err := gitCommand(folder, "ls-files", "--stage", "-z").Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	return parseGitIndexModes(out)
}

// parseGitIndexModes parses `git ls-files --stage -z` output, which is a
// sequence of NUL-terminated "<mode> <object> <stage>\t<path>" entries.
func parseGitIndexModes(out []byte) (map[string]string, error) {
	modes := map[string]string{}
	for _, entry := range bytes.Split(out, []byte{0}) {
		if len(entry) == 0 {
			continue
		}

		meta, path, ok := strings.Cut(string(entry), "\t")
		if !ok {
			return nil, fmt.Errorf("malformed git ls-files entry: %q", entry)
		}
		mode, _, _ := strings.Cut(meta, " ")
		modes[path] = mode
	}
	return modes, nil
}
