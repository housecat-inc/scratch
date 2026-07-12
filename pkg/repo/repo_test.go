package repo

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScan(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	home := t.TempDir()
	r.NoError(os.MkdirAll(filepath.Join(home, "not-a-repo"), 0o755))
	r.NoError(os.WriteFile(filepath.Join(home, "loose-file"), []byte("x"), 0o644))

	makeRepo(t, filepath.Join(home, "alpha"), "https://example.com/acme/alpha.git")
	makeRepo(t, filepath.Join(home, "beta"), "https://example.com/acme/beta.git")

	repos, err := Scan(home)
	r.NoError(err)
	r.Len(repos, 2)
	a.Equal("acme/alpha", repos[0].Slug())
	a.Equal("acme/beta", repos[1].Slug())
	a.NotEmpty(repos[0].LastCommit.SHA)
	a.NotEmpty(repos[0].Branch)
}

func TestScanSkipsReposWithoutOrigin(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	home := t.TempDir()
	path := filepath.Join(home, "no-origin")
	r.NoError(os.MkdirAll(path, 0o755))
	runGit(t, path, "init", "--initial-branch=main")
	runGit(t, path, "config", "user.email", "t@t.t")
	runGit(t, path, "config", "user.name", "t")
	r.NoError(os.WriteFile(filepath.Join(path, "README"), []byte("x"), 0o644))
	runGit(t, path, "add", ".")
	runGit(t, path, "commit", "-m", "init")

	repos, err := Scan(home)
	r.NoError(err)
	a.Empty(repos)
}

func TestScanIncludingPrefersExplicitWorktree(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	home := t.TempDir()
	main := filepath.Join(home, "scratch")
	makeRepo(t, main, "https://github.com/housecat-inc/scratch.git")
	worktree := filepath.Join(t.TempDir(), "conductor", "workspaces", "scratch", "newport-beach")
	runGit(t, main, "worktree", "add", "-b", "feature", worktree)

	repos, err := ScanIncluding(home, worktree)
	r.NoError(err)
	r.Len(repos, 1)
	a.Equal("housecat-inc/scratch", repos[0].Slug())
	a.Equal(worktree, repos[0].Path)

	got, ok := FindIncluding(home, "housecat-inc/scratch", worktree)
	r.True(ok)
	a.Equal(worktree, got.Path)
}

func TestFind(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	home := t.TempDir()
	makeRepo(t, filepath.Join(home, "alpha"), "https://example.com/acme/alpha.git")

	got, ok := Find(home, "acme/alpha")
	r.True(ok)
	a.Equal(filepath.Join(home, "alpha"), got.Path)

	_, ok = Find(home, "acme/missing")
	a.False(ok)
}

func makeRepo(t *testing.T, path, remoteURL string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(path, 0o755))
	runGit(t, path, "init", "--initial-branch=main")
	runGit(t, path, "config", "user.email", "t@t.t")
	runGit(t, path, "config", "user.name", "t")
	runGit(t, path, "config", "commit.gpgsign", "false")
	require.NoError(t, os.WriteFile(filepath.Join(path, "README"), []byte("x"), 0o644))
	runGit(t, path, "add", ".")
	runGit(t, path, "commit", "-m", "init")
	runGit(t, path, "remote", "add", "origin", remoteURL)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t.t",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t.t",
	)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
}
