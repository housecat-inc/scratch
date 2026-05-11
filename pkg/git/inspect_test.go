package git

import (
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSlug(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		org     string
		repo    string
		wantErr bool
	}{
		{name: "https github", url: "https://github.com/foo/bar.git", org: "foo", repo: "bar"},
		{name: "https github no suffix", url: "https://github.com/foo/bar", org: "foo", repo: "bar"},
		{name: "ssh shorthand", url: "git@github.com:foo/bar.git", org: "foo", repo: "bar"},
		{name: "ssh full", url: "ssh://git@github.com/foo/bar.git", org: "foo", repo: "bar"},
		{name: "internal mirror", url: "https://housecat-inc-scratch.int.exe.xyz/housecat-inc/scratch.git", org: "housecat-inc", repo: "scratch"},
		{name: "trailing slash", url: "https://github.com/foo/bar/", org: "foo", repo: "bar"},
		{name: "missing repo", url: "https://github.com/foo", wantErr: true},
		{name: "empty", url: "", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			org, repo, err := parseSlug(tc.url)
			if tc.wantErr {
				r.Error(err)
				return
			}
			r.NoError(err)
			a.Equal(tc.org, org)
			a.Equal(tc.repo, repo)
		})
	}
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
