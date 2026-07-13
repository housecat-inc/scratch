package workspace

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveInRepo(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "t@x.com"},
		{"config", "user.name", "t"},
		{"checkout", "-q", "-b", "feature/x"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		r.NoError(cmd.Run(), "git %v", args)
	}

	sub := filepath.Join(dir, "pkg")
	r.NoError(os.MkdirAll(sub, 0o755))
	t.Chdir(sub)

	w, err := Resolve()
	r.NoError(err)
	a.True(w.IsRepo)
	a.Equal("feature/x", w.Branch)
	a.NotEmpty(w.Home)

	top, err := filepath.EvalSymlinks(dir)
	r.NoError(err)
	a.Equal(top, w.Dir, "Dir is the worktree root, not the cwd")
}

func TestResolveOutsideRepo(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	t.Chdir(dir)

	w, err := Resolve()
	r.NoError(err)
	a.False(w.IsRepo)
	a.Empty(w.Branch)
	a.NotEmpty(w.Dir)
}
