package git

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranch(t *testing.T) {
	t.Run("returns the checked out branch", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)
		runGit(t, clone, "checkout", "-b", "feature")

		b, err := Branch(clone)
		r.NoError(err)
		a.Equal("feature", b)
	})
}

func TestCommitLog(t *testing.T) {
	t.Run("lists branch commits newest first", func(t *testing.T) {
		requireGit(t)
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
	})
}

func TestDefaultBranch(t *testing.T) {
	t.Run("returns the remote head", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)
		runGit(t, clone, "remote", "set-head", "origin", "main")

		b, err := DefaultBranch(clone)
		r.NoError(err)
		a.Equal("origin/main", b)
	})
}

func TestHunkCommit(t *testing.T) {
	t.Run("finds the commit that introduced a hunk", func(t *testing.T) {
		requireGit(t)
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
	})
}

func TestLastCommit(t *testing.T) {
	t.Run("returns the head subject sha and date", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)

		c, err := LastCommit(clone)
		r.NoError(err)
		a.Equal("initial", c.Subject)
		a.NotEmpty(c.SHA)
		a.False(c.Date.IsZero())
	})
}

func TestRemoteSlug(t *testing.T) {
	t.Run("derives the repo name from origin", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)

		_, name, err := RemoteSlug(clone)
		r.NoError(err)
		a.NotEmpty(name)
	})
}

func TestShowFile(t *testing.T) {
	t.Run("returns the file lines at a revision", func(t *testing.T) {
		requireGit(t)
		a := assert.New(t)
		r := require.New(t)

		clone, _ := setupRepo(t)
		r.NoError(writeFile(clone+"/multi.txt", "a\nb\nc\n"))
		runGit(t, clone, "add", ".")
		runGit(t, clone, "commit", "-m", "multi")

		lines, err := ShowFile(clone, "HEAD", "multi.txt")
		r.NoError(err)
		a.Equal([]string{"a", "b", "c"}, lines)
	})
}

func TestParseSlug(t *testing.T) {
	tests := []struct {
		err  string
		in   string
		name string
		org  string
		repo string
	}{
		{in: "https://github.com/foo/bar.git", name: "parses an https github url", org: "foo", repo: "bar"},
		{in: "https://github.com/foo/bar", name: "parses an https url without the git suffix", org: "foo", repo: "bar"},
		{in: "git@github.com:foo/bar.git", name: "parses ssh shorthand", org: "foo", repo: "bar"},
		{in: "ssh://git@github.com/foo/bar.git", name: "parses a full ssh url", org: "foo", repo: "bar"},
		{in: "https://housecat-inc-scratch.int.exe.xyz/housecat-inc/scratch.git", name: "parses an internal mirror url", org: "housecat-inc", repo: "scratch"},
		{in: "https://github.com/foo/bar/", name: "ignores a trailing slash", org: "foo", repo: "bar"},
		{err: "bad remote url", in: "https://github.com/foo", name: "rejects a url without a repo"},
		{err: "bad remote url", in: "", name: "rejects an empty url"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			org, repo, err := parseSlug(tt.in)
			if tt.err != "" {
				r.ErrorContains(err, tt.err)
				return
			}
			r.NoError(err)
			a.Equal(tt.org, org)
			a.Equal(tt.repo, repo)
		})
	}
}

func writeFile(path, content string) error {
	return os.WriteFile(path, []byte(content), 0o644)
}
