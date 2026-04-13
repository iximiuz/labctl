package ide

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRemotePathIsAbs(t *testing.T) {
	assert.True(t, remotePathIsAbs("/home/laborant"))
	assert.True(t, remotePathIsAbs("/root"))
	assert.False(t, remotePathIsAbs("home/laborant"))
	assert.False(t, remotePathIsAbs("./home/laborant"))
}

func TestRemotePathJoin(t *testing.T) {
	assert.Equal(t, "/home/laborant/projects", remotePathJoin("/home/laborant", "projects"))
	assert.Equal(t, "/home/laborant", remotePathJoin("/home/laborant", "/home/laborant"))
	assert.Equal(t, "/home/laborant/home/laborant", remotePathJoin("/home/laborant", "home/laborant"))
}

func TestRepoSpecCloneTarget(t *testing.T) {
	baseDir := "/home/laborant"

	repo := repoSpec{url: "https://github.com/foo/bar"}
	assert.Equal(t, "/home/laborant/bar", repo.cloneTarget(baseDir))

	repo = repoSpec{url: "git@github.com:foo/bar", cloneDir: "projects"}
	assert.Equal(t, "/home/laborant/projects", repo.cloneTarget(baseDir))

	repo = repoSpec{url: "git@github.com:foo/bar", cloneDir: "/tmp/project"}
	assert.Equal(t, "/tmp/project", repo.cloneTarget(baseDir))

	repo = repoSpec{url: "https://github.com/foo/bar", cloneDir: "home/laborant"}
	assert.Equal(t, "/home/laborant/home/laborant", repo.cloneTarget(baseDir))
}
