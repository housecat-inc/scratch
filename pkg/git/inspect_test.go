package git

import (
	"os"
	"os/exec"
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSlug(t *testing.T) {
	type out struct{ org, repo string }

	tk.Run(t, []tk.Test[string, out]{
		{Name: "https github", In: "https://github.com/foo/bar.git", Out: out{org: "foo", repo: "bar"}},
		{Name: "https github no suffix", In: "https://github.com/foo/bar", Out: out{org: "foo", repo: "bar"}},
		{Name: "ssh shorthand", In: "git@github.com:foo/bar.git", Out: out{org: "foo", repo: "bar"}},
		{Name: "ssh full", In: "ssh://git@github.com/foo/bar.git", Out: out{org: "foo", repo: "bar"}},
		{Name: "internal mirror", In: "https://housecat-inc-scratch.int.exe.xyz/housecat-inc/scratch.git", Out: out{org: "housecat-inc", repo: "scratch"}},
		{Name: "trailing slash", In: "https://github.com/foo/bar/", Out: out{org: "foo", repo: "bar"}},
		{Name: "missing repo", In: "https://github.com/foo", Err: "bad remote url"},
		{Name: "empty", In: "", Err: "bad remote url"},
	}, func(url string) (out, error) {
		org, repo, err := parseSlug(url)
		return out{org: org, repo: repo}, err
	})
}

func TestBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	runGit(t, clone, "checkout", "-b", "feature")

	b, err := Branch(clone)
	r.NoError(err)
	a.Equal("feature", b)
}

func TestDefaultBranch(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	runGit(t, clone, "remote", "set-head", "origin", "main")

	b, err := DefaultBranch(clone)
	r.NoError(err)
	a.Equal("origin/main", b)
}

func TestLastCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)

	c, err := LastCommit(clone)
	r.NoError(err)
	a.Equal("initial", c.Subject)
	a.NotEmpty(c.SHA)
	a.False(c.Date.IsZero())
}

func TestCommitLog(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	runGit(t, clone, "checkout", "-b", "feature")
	r.NoError(writeFile(clone+"/a.txt", "a\n"))
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "-m", "add a")
	r.NoError(writeFile(clone+"/b.txt", "b\n"))
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "-m", "add b")

	cs, err := CommitLog(clone, "main", "HEAD")
	r.NoError(err)
	r.Len(cs, 2)
	a.Equal("add b", cs[0].Subject)
	a.Equal("add a", cs[1].Subject)

	none, err := CommitLog(clone, "HEAD", "HEAD")
	r.NoError(err)
	a.Empty(none)
}

func TestHunkCommit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	runGit(t, clone, "checkout", "-b", "feature")
	r.NoError(writeFile(clone+"/README", "v1\nadded\n"))
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "-m", "introduce added line")

	c, err := HunkCommit(clone, "main", "HEAD", "README", 2, 1)
	r.NoError(err)
	a.Equal("introduce added line", c.Subject)
	a.NotEmpty(c.SHA)
}

func TestShowFile(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)
	r.NoError(writeFile(clone+"/multi.txt", "a\nb\nc\n"))
	runGit(t, clone, "add", ".")
	runGit(t, clone, "commit", "-m", "multi")

	lines, err := ShowFile(clone, "HEAD", "multi.txt")
	r.NoError(err)
	a.Equal([]string{"a", "b", "c"}, lines)
}

func TestRemoteSlug(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	a := assert.New(t)
	r := require.New(t)

	clone, _ := setupRepo(t)

	org, name, err := RemoteSlug(clone)
	r.NoError(err)
	a.NotEmpty(name)
	// setupRepo uses a local bare repo as origin; org is the parent dir name.
	_ = org
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
