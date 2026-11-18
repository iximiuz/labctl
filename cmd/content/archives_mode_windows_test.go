//go:build windows

package content

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func entriesFor(names ...string) []archiveEntry {
	var entries []archiveEntry
	for _, name := range names {
		entries = append(entries, archiveEntry{name: name})
	}
	return entries
}

func TestArchiveFileModes_GitIndex(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required")
	}

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "run.sh"), "#!/bin/sh\n", 0644)
	writeFile(t, filepath.Join(dir, "sub", "data.txt"), "data", 0644)
	writeFile(t, filepath.Join(dir, "Tool.sh"), "#!/bin/sh\n", 0644)
	writeFile(t, filepath.Join(dir, "new.sh"), "#!/bin/sh\n", 0644)

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "run.sh", "sub/data.txt", "Tool.sh"},
		{"update-index", "--chmod=+x", "run.sh", "Tool.sh"},
	} {
		out, err := gitCommand(dir, args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}

	// "tool.sh" simulates a case-only rename of the indexed "Tool.sh".
	modes := archiveFileModes(dir, entriesFor("run.sh", "sub/data.txt", "tool.sh", "new.sh"))

	assert.Equal(t, map[string]int64{
		"run.sh":       0755,
		"sub/data.txt": 0644,
		"tool.sh":      0755,
		"new.sh":       0644, // untracked
	}, modes)
}

func TestArchiveFileModes_NoGitRepo(t *testing.T) {
	// Keep git from discovering a repository above the temp dir.
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(t.TempDir()))

	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "run.sh"), "#!/bin/sh\n", 0755)

	modes := archiveFileModes(dir, entriesFor("run.sh"))
	assert.Equal(t, map[string]int64{"run.sh": 0644}, modes)
}
