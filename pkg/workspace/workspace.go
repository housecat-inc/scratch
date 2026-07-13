package workspace

import (
	"os"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
)

// Workspace is the single source of truth for where the app operates: the
// worktree it browses, edits, reviews, and launches sessions in.
type Workspace struct {
	Base   string // default branch reviews compare against (e.g. origin/main)
	Branch string // current branch of the worktree
	Dir    string // worktree root (git toplevel), or the cwd when not a repo
	Home   string // user home directory
	IsRepo bool   // whether Dir is a git worktree
}

// Resolve inspects the current working directory once and decides the
// working directory, worktree root, and branch for the whole app.
func Resolve() (Workspace, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Workspace{}, errors.Wrap(err, "user home dir")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return Workspace{}, errors.Wrap(err, "working dir")
	}

	w := Workspace{Dir: cwd, Home: home}
	if root, err := git.Toplevel(cwd); err == nil && root != "" {
		w.Dir = root
		w.IsRepo = true
		if b, err := git.Branch(root); err == nil {
			w.Branch = b
		}
		if base, err := git.DefaultBranch(root); err == nil {
			w.Base = base
		}
	}
	return w, nil
}
