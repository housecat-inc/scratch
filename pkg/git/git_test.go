package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstalled(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, dir string)
		want  bool
	}{
		{name: "missing dir is not installed"},
		{
			name: "dir without .git is not installed",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(dir, 0o755))
			},
		},
		{
			name: "dir with .git is installed",
			setup: func(t *testing.T, dir string) {
				require.NoError(t, os.MkdirAll(filepath.Join(dir, ".git"), 0o755))
			},
			want: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			dir := filepath.Join(t.TempDir(), "repo")
			if tc.setup != nil {
				tc.setup(t, dir)
			}
			ok, err := Installed(dir)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}

func TestInstalledRecognizesWorktreeFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	worktree := filepath.Join(t.TempDir(), "feature")
	runGit(t, clone, "worktree", "add", "-b", "feature", worktree)

	ok, err := Installed(worktree)
	r.NoError(err)
	a.True(ok)
}

func TestComputeStatus(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}

	tests := []struct {
		name     string
		mutate   func(t *testing.T, clone, remote string)
		dirty    bool
		diverged bool
		behind   int
	}{
		{name: "clean and current"},
		{
			name: "behind by one",
			mutate: func(t *testing.T, clone, remote string) {
				up := makeUpstream(t, remote)
				gitCommit(t, up, "remote-2")
				runGit(t, up, "push", "origin", "main")
			},
			behind: 1,
		},
		{
			name: "dirty working tree",
			mutate: func(t *testing.T, clone, remote string) {
				require.NoError(t, os.WriteFile(filepath.Join(clone, "README"), []byte("edit"), 0o644))
			},
			dirty: true,
		},
		{
			name: "diverged ahead",
			mutate: func(t *testing.T, clone, remote string) {
				gitCommit(t, clone, "local-2")
				up := makeUpstream(t, remote)
				gitCommit(t, up, "remote-2")
				runGit(t, up, "push", "origin", "main")
			},
			behind:   1,
			diverged: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			clone, remote := setupRepo(t)
			if tc.mutate != nil {
				tc.mutate(t, clone, remote)
			}
			runGit(t, clone, "fetch", "origin", "main")

			state, err := ComputeStatus(clone, "main")
			r.NoError(err)
			a.True(state.Installed)
			a.Equal(tc.behind, state.Behind)
			a.Equal(tc.dirty, state.Dirty)
			a.Equal(tc.diverged, state.Diverged)
		})
	}
}

func TestPull(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, remote := setupRepo(t)
	up := makeUpstream(t, remote)
	gitCommit(t, up, "remote-2")
	runGit(t, up, "push", "origin", "main")

	r.NoError(Pull(clone, "main"))

	a.FileExists(filepath.Join(clone, "remote-2"))
}

func setupRepo(t *testing.T) (clone, remote string) {
	t.Helper()
	root := t.TempDir()
	remote = filepath.Join(root, "remote.git")
	runGit(t, "", "init", "--bare", "--initial-branch=main", remote)

	seed := filepath.Join(root, "seed")
	require.NoError(t, os.MkdirAll(seed, 0o755))
	runGit(t, seed, "init", "--initial-branch=main")
	configRepo(t, seed)
	require.NoError(t, os.WriteFile(filepath.Join(seed, "README"), []byte("v1"), 0o644))
	runGit(t, seed, "add", ".")
	runGit(t, seed, "commit", "-m", "initial")
	runGit(t, seed, "remote", "add", "origin", remote)
	runGit(t, seed, "push", "origin", "main")

	clone = filepath.Join(root, "clone")
	runGit(t, "", "clone", remote, clone)
	configRepo(t, clone)
	return clone, remote
}

func makeUpstream(t *testing.T, remote string) string {
	t.Helper()
	work := filepath.Join(t.TempDir(), "upstream")
	runGit(t, "", "clone", remote, work)
	configRepo(t, work)
	return work
}

func gitCommit(t *testing.T, dir, msg string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, msg), []byte(msg), 0o644))
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", msg)
}

func configRepo(t *testing.T, dir string) {
	t.Helper()
	runGit(t, dir, "config", "user.email", "test@test.test")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "config", "commit.gpgsign", "false")
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test.test",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test.test",
	)
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "git %v: %s", args, out)
}
