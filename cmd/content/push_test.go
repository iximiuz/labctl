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
		relPaths = append(relPaths, relPath)
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
		relPaths = append(relPaths, relPath)
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
