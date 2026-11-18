package content

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// readTarGz reads a (possibly gzipped) tar archive and returns its entries
// keyed by name.
func readTarGz(t *testing.T, path string, gzipped bool) map[string]*tar.Header {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var r io.Reader = bytes.NewReader(data)
	if gzipped {
		gz, err := gzip.NewReader(bytes.NewReader(data))
		require.NoError(t, err)
		defer gz.Close()
		r = gz
	}

	tr := tar.NewReader(r)
	entries := map[string]*tar.Header{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		hdr.Name = filepath.ToSlash(hdr.Name)
		entries[hdr.Name] = hdr
	}
	return entries
}

func writeFile(t *testing.T, path, content string, mode os.FileMode) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), mode))
}

// markExecutable makes file (relative to dir) executable in the eyes of the
// archiver. Windows has no executable bit, so it's recorded in the git index.
func markExecutable(t *testing.T, dir, file string) {
	t.Helper()
	if runtime.GOOS != "windows" {
		return
	}

	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is required to track the executable bit on Windows")
	}

	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", file},
		{"update-index", "--chmod=+x", file},
	} {
		out, err := gitCommand(dir, args...).CombinedOutput()
		require.NoError(t, err, string(out))
	}
}

func TestParseGitIndexModes(t *testing.T) {
	out := []byte("100644 0123456789abcdef0123456789abcdef01234567 0\tmain.go\x00" +
		"100755 0123456789abcdef0123456789abcdef01234567 0\tsub dir/run.sh\x00")

	modes, err := parseGitIndexModes(out)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		"main.go":        "100644",
		"sub dir/run.sh": "100755",
	}, modes)

	modes, err = parseGitIndexModes(nil)
	require.NoError(t, err)
	assert.Empty(t, modes)

	_, err = parseGitIndexModes([]byte("garbage\x00"))
	assert.Error(t, err)
}

func TestCreateStartupFileArchives_MarkdownFrontMatter(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "index.md"), `---
title: Test
playground:
  startupFiles:
    - path: /home/laborant/app
      source: __static__/app.tar.gz
      extract: true
---

# Hello
`, 0644)

	writeFile(t, filepath.Join(tmpDir, "app", "main.go"), "package main\n", 0644)
	writeFile(t, filepath.Join(tmpDir, "app", "sub", "x.txt"), "hello", 0644)
	writeFile(t, filepath.Join(tmpDir, "app", "run.sh"), "#!/bin/sh\n", 0755)
	markExecutable(t, tmpDir, filepath.Join("app", "run.sh"))

	folders, err := createStartupFileArchives(tmpDir)
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(tmpDir, "app")}, folders)

	archivePath := filepath.Join(tmpDir, "__static__", "app.tar.gz")
	require.FileExists(t, archivePath)

	entries := readTarGz(t, archivePath, true)
	require.Len(t, entries, 3)

	main, ok := entries["main.go"]
	require.True(t, ok)
	assert.Equal(t, int64(0644), main.Mode)

	sub, ok := entries["sub/x.txt"]
	require.True(t, ok)
	assert.Equal(t, int64(0644), sub.Mode)

	run, ok := entries["run.sh"]
	require.True(t, ok)
	assert.Equal(t, int64(0755), run.Mode)

	for _, hdr := range entries {
		assert.Equal(t, int64(0), int64(hdr.Uid))
		assert.Equal(t, int64(0), int64(hdr.Gid))
		assert.Equal(t, "", hdr.Uname)
		assert.Equal(t, "", hdr.Gname)
		assert.True(t, hdr.ModTime.Equal(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)))
	}

	// Verify file contents.
	data, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	gz, err := gzip.NewReader(bytes.NewReader(data))
	require.NoError(t, err)
	tr := tar.NewReader(gz)
	contents := map[string]string{}
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)
		buf, err := io.ReadAll(tr)
		require.NoError(t, err)
		contents[filepath.ToSlash(hdr.Name)] = string(buf)
	}
	assert.Equal(t, "package main\n", contents["main.go"])
	assert.Equal(t, "hello", contents["sub/x.txt"])
	assert.Equal(t, "#!/bin/sh\n", contents["run.sh"])

	// Calling again with unchanged sources must not rewrite the archive.
	statBefore, err := os.Stat(archivePath)
	require.NoError(t, err)

	folders2, err := createStartupFileArchives(tmpDir)
	require.NoError(t, err)
	require.Equal(t, folders, folders2)

	statAfter, err := os.Stat(archivePath)
	require.NoError(t, err)
	assert.Equal(t, statBefore.ModTime(), statAfter.ModTime())

	dataAfter, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	assert.Equal(t, data, dataAfter)
}

func TestCreateStartupFileArchives_Manifest(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "manifest.yaml"), `
startupFiles:
  - path: /x
    source: __static__/tools.tgz
machines:
  - name: dev-machine
    startupFiles:
      - path: /y
        source: __static__/data.tar
`, 0644)

	writeFile(t, filepath.Join(tmpDir, "tools", "a.txt"), "a", 0644)
	writeFile(t, filepath.Join(tmpDir, "data", "b.txt"), "b", 0644)

	folders, err := createStartupFileArchives(tmpDir)
	require.NoError(t, err)
	assert.ElementsMatch(t, []string{
		filepath.Join(tmpDir, "tools"),
		filepath.Join(tmpDir, "data"),
	}, folders)

	toolsArchive := filepath.Join(tmpDir, "__static__", "tools.tgz")
	require.FileExists(t, toolsArchive)
	toolsEntries := readTarGz(t, toolsArchive, true)
	require.Contains(t, toolsEntries, "a.txt")

	dataArchive := filepath.Join(tmpDir, "__static__", "data.tar")
	require.FileExists(t, dataArchive)

	// data.tar must not be gzipped - it should parse fine as a plain tar,
	// and fail to parse as gzip.
	rawData, err := os.ReadFile(dataArchive)
	require.NoError(t, err)
	_, gzErr := gzip.NewReader(bytes.NewReader(rawData))
	assert.Error(t, gzErr, "data.tar should not be gzip-compressed")

	dataEntries := readTarGz(t, dataArchive, false)
	require.Contains(t, dataEntries, "b.txt")
}

func TestCreateStartupFileArchives_NoOpCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Scalar `playground:` front matter - no error, no archives.
	writeFile(t, filepath.Join(tmpDir, "scalar.md"), `---
title: Test
playground: docker
---

# Hello
`, 0644)

	// Markdown without front matter at all.
	writeFile(t, filepath.Join(tmpDir, "plain.md"), "# Just a heading\n", 0644)

	// Source whose folder doesn't exist.
	writeFile(t, filepath.Join(tmpDir, "missing.md"), `---
playground:
  startupFiles:
    - path: /x
      source: __static__/ghost.tar.gz
---
`, 0644)

	// Source pointing into a subfolder - never matches.
	writeFile(t, filepath.Join(tmpDir, "nested.md"), `---
playground:
  startupFiles:
    - path: /x
      source: __static__/nested/app.tar.gz
---
`, 0644)
	writeFile(t, filepath.Join(tmpDir, "nested", "app", "f.txt"), "f", 0644)

	folders, err := createStartupFileArchives(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, folders)

	_, err = os.Stat(filepath.Join(tmpDir, "__static__"))
	assert.True(t, os.IsNotExist(err), "no __static__ dir should have been created")
}

func TestCreateStartupFileArchives_Reproducible(t *testing.T) {
	makeDir := func() string {
		dir := t.TempDir()
		writeFile(t, filepath.Join(dir, "index.md"), `---
playground:
  startupFiles:
    - path: /x
      source: __static__/app.tar.gz
---
`, 0644)
		writeFile(t, filepath.Join(dir, "app", "main.go"), "package main\n", 0644)
		writeFile(t, filepath.Join(dir, "app", "sub", "x.txt"), "hello", 0644)
		writeFile(t, filepath.Join(dir, "app", "run.sh"), "#!/bin/sh\n", 0755)
		return dir
	}

	dir1 := makeDir()
	dir2 := makeDir()

	_, err := createStartupFileArchives(dir1)
	require.NoError(t, err)
	_, err = createStartupFileArchives(dir2)
	require.NoError(t, err)

	data1, err := os.ReadFile(filepath.Join(dir1, "__static__", "app.tar.gz"))
	require.NoError(t, err)
	data2, err := os.ReadFile(filepath.Join(dir2, "__static__", "app.tar.gz"))
	require.NoError(t, err)

	assert.Equal(t, data1, data2)
}

func TestCreateStartupFileArchives_RespectsFolderIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	writeFile(t, filepath.Join(tmpDir, "index.md"), `---
playground:
  startupFiles:
    - path: /x
      source: __static__/app.tar.gz
---
`, 0644)

	writeFile(t, filepath.Join(tmpDir, "app", "keep.txt"), "keep", 0644)
	writeFile(t, filepath.Join(tmpDir, "app", "secret.txt"), "secret", 0644)
	writeFile(t, filepath.Join(tmpDir, "app", ".labctlignore"), "secret.txt\n", 0644)

	folders, err := createStartupFileArchives(tmpDir)
	require.NoError(t, err)
	require.Equal(t, []string{filepath.Join(tmpDir, "app")}, folders)

	entries := readTarGz(t, filepath.Join(tmpDir, "__static__", "app.tar.gz"), true)
	assert.Contains(t, entries, "keep.txt")
	assert.NotContains(t, entries, "secret.txt")
	assert.NotContains(t, entries, ".labctlignore")
}

func TestIsCRLFScript(t *testing.T) {
	assert.True(t, isCRLFScript([]byte("#!/bin/sh\r\necho hi\r\n")))
	assert.True(t, isCRLFScript([]byte("#!/bin/sh\r")))
	assert.False(t, isCRLFScript([]byte("#!/bin/sh\necho hi\n")))
	assert.False(t, isCRLFScript([]byte("#!/bin/sh")))
	assert.False(t, isCRLFScript([]byte("plain text\r\n")))
	assert.False(t, isCRLFScript(nil))
}
