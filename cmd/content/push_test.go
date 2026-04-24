package content

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFiles(t *testing.T) {
	// Setup test directory structure
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── file1.txt
	//   ├── file2.txt
	//   ├── .git/
	//   │   ├── config
	//   │   └── objects/
	//   │       └── test.obj
	//   └── subdir/
	//       ├── file3.txt
	//       └── .git/
	//           └── config

	files := map[string]string{
		"file1.txt":             "content1",
		"file2.txt":             "content2",
		".git/config":           "git config",
		".git/objects/test.obj": "test object",
		"subdir/file3.txt":      "content3",
		"subdir/.git/config":    "subdir git config",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		require.NoError(t, os.MkdirAll(filepath.Dir(fullPath), 0755))
		require.NoError(t, os.WriteFile(fullPath, []byte(content), 0644))
	}

	// Test listFiles
	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	// Convert results to relative paths for easier testing
	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// Expected files (only non-.git files)
	expected := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
	}

	// Sort both slices for consistent comparison
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)

	// Verify .git files are not included
	for _, path := range relPaths {
		assert.NotContains(t, path, ".git")
	}
}

func TestListDirs(t *testing.T) {
	// Setup test directory structure
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── .git/
	//   │   └── objects/
	//   ├── dir1/
	//   │   └── subdir/
	//   └── dir2/
	//       └── .git/

	dirs := []string{
		".git/objects",
		"dir1/subdir",
		"dir2/.git",
	}

	for _, dir := range dirs {
		require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, dir), 0755))
	}

	// Test listDirs
	result, err := listDirs(tmpDir)
	require.NoError(t, err)

	// Convert results to relative paths for easier testing
	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// Expected directories (only non-.git directories)
	expected := []string{
		"dir1",
		"dir1/subdir",
		"dir2",
	}

	// Sort both slices for consistent comparison
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)

	// Verify .git directories are not included
	for _, path := range relPaths {
		assert.NotContains(t, path, ".git")
	}
}

func TestListFilesEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := listFiles(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListDirsEmptyDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	result, err := listDirs(tmpDir)
	require.NoError(t, err)
	assert.Empty(t, result)
}

func TestListFilesIgnoreBackupFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file and a ~ temp file
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("content"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md~"), []byte("unsaved content"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	// assert: the temp file is not listed
	assert.Equal(t, []string{"index.md"}, relPaths)
}

func TestListFilesLabctlIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test structure:
	// tmpDir/
	//   ├── .labctlignore
	//   ├── index.md
	//   ├── CLAUDE.md        <- ignored by exact name pattern
	//   ├── AGENTS.md        <- ignored by exact name pattern
	//   ├── notes.txt
	//   ├── .omc/
	//   │   └── config       <- ignored because .omc/ dir is ignored
	//   └── subdir/
	//       ├── CLAUDE.md    <- ignored by exact name (matches basename)
	//       └── readme.md

	ignoreContent := "# Claude-specific files\nCLAUDE.md\nAGENTS.md\n.omc/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("claude"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "AGENTS.md"), []byte("agents"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("notes"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".omc"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".omc/config"), []byte("omc config"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/CLAUDE.md"), []byte("claude"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/readme.md"), []byte("readme"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"notes.txt",
		"subdir/readme.md",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListFilesLabctlIgnoreGlobPattern(t *testing.T) {
	tmpDir := t.TempDir()

	ignoreContent := "*.log\nbuild/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "debug.log"), []byte("log"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "build/output.bin"), []byte("bin"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/main.go"), []byte("code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "src/trace.log"), []byte("log"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"src/main.go",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListFilesLabctlIgnoreCascading(t *testing.T) {
	// Root .labctlignore excludes CLAUDE.md everywhere.
	// subdir/.labctlignore additionally excludes *.log within subdir and below.
	tmpDir := t.TempDir()

	// Root structure
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte("CLAUDE.md\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "index.md"), []byte("index"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "CLAUDE.md"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "root.log"), []byte("log"), 0644)) // NOT ignored at root level

	// subdir with its own .labctlignore
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/.labctlignore"), []byte("*.log\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/main.go"), []byte("code"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/debug.log"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/CLAUDE.md"), []byte("ignored"), 0644)) // root rule cascades

	// nested inside subdir — both rule sets should apply
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "subdir/nested"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/notes.md"), []byte("notes"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/trace.log"), []byte("ignored"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "subdir/nested/CLAUDE.md"), []byte("ignored"), 0644))

	result, err := listFiles(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{
		"index.md",
		"root.log",
		"subdir/main.go",
		"subdir/nested/notes.md",
	}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}

func TestListDirsLabctlIgnore(t *testing.T) {
	tmpDir := t.TempDir()

	ignoreContent := ".omc/\nbuild/\n"
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, ".labctlignore"), []byte(ignoreContent), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, ".omc"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "build"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "src/lib"), 0755))

	result, err := listDirs(tmpDir)
	require.NoError(t, err)

	var relPaths []string
	for _, path := range result {
		relPath, err := filepath.Rel(tmpDir, path)
		require.NoError(t, err)
		relPaths = append(relPaths, filepath.ToSlash(relPath))
	}

	expected := []string{"src", "src/lib"}
	slices.Sort(relPaths)
	slices.Sort(expected)

	assert.Equal(t, expected, relPaths)
}
