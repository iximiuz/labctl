package skills

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFindProject(t *testing.T) {
	tmp := t.TempDir()
	mkdir := func(parts ...string) string {
		dir := filepath.Join(append([]string{tmp}, parts...)...)
		require.NoError(t, os.MkdirAll(dir, 0o755))
		return dir
	}
	touch := func(dir, name string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), nil, 0o644))
	}

	// A repository with the skills installed at its root.
	repo := mkdir("repo")
	mkdir("repo", ".git")
	touch(repo, projectManifest)
	nested := mkdir("repo", "3rd-party", "challenges", "foo")

	root, installed := findProject(nested)
	assert.Equal(t, repo, root)
	assert.True(t, installed)

	// The manifest inside the content folder itself doesn't count.
	touch(nested, projectManifest)
	root, installed = findProject(nested)
	assert.Equal(t, repo, root)
	assert.True(t, installed)

	// A repository without the skills: suggest its root.
	bare := mkdir("bare")
	mkdir("bare", ".git")
	content := mkdir("bare", "challenges", "bar")

	root, installed = findProject(content)
	assert.Equal(t, bare, root)
	assert.False(t, installed)

	// The search stops at the repository root.
	touch(tmp, projectManifest)
	root, installed = findProject(content)
	assert.Equal(t, bare, root)
	assert.False(t, installed)
	require.NoError(t, os.Remove(filepath.Join(tmp, projectManifest)))

	// No repository and no skills: suggest the content folder's parent.
	plain := mkdir("plain", "challenge-baz")
	root, installed = findProject(plain)
	assert.Equal(t, filepath.Dir(plain), root)
	assert.False(t, installed)
}
