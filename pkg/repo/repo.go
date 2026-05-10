package repo

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
)

type Repo struct {
	Base       string
	Branch     string
	LastCommit git.Commit
	Name       string
	Org        string
	Path       string
	State      git.State
}

func (r Repo) Slug() string { return r.Org + "/" + r.Name }

func Scan(home string) ([]Repo, error) {
	entries, err := os.ReadDir(home)
	if err != nil {
		return nil, errors.Wrapf(err, "read dir %s", home)
	}
	var repos []Repo
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(home, e.Name())
		installed, err := git.Installed(path)
		if err != nil || !installed {
			continue
		}
		r, ok := Inspect(path)
		if !ok {
			continue
		}
		repos = append(repos, r)
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Slug() < repos[j].Slug() })
	return repos, nil
}

func Inspect(path string) (Repo, bool) {
	org, name, err := git.RemoteSlug(path)
	if err != nil {
		return Repo{}, false
	}
	r := Repo{Name: name, Org: org, Path: path}
	if b, err := git.Branch(path); err == nil {
		r.Branch = b
	}
	if c, err := git.LastCommit(path); err == nil {
		r.LastCommit = c
	}
	if base, err := git.DefaultBranch(path); err == nil {
		r.Base = base
		short := strings.TrimPrefix(base, "origin/")
		if s, err := git.ComputeStatus(path, short); err == nil {
			r.State = s
		}
	}
	return r, true
}

func Find(home, slug string) (Repo, bool) {
	repos, err := Scan(home)
	if err != nil {
		return Repo{}, false
	}
	for _, r := range repos {
		if r.Slug() == slug {
			return r, true
		}
	}
	return Repo{}, false
}
