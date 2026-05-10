package agents

import (
	"os"
	"os/exec"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
)

const (
	repoBranch = "main"
	repoSlug   = "housecat-inc/scratch"
	repoURL    = "https://github.com/housecat-inc/scratch.git"
)

type State = git.State

func Dir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", errors.Wrap(err, "user home dir")
	}
	return filepath.Join(home, "scratch"), nil
}

func Installed() (bool, error) {
	dir, err := Dir()
	if err != nil {
		return false, err
	}
	return git.Installed(dir)
}

func InstalledAt(dir string) (bool, error) {
	return git.Installed(dir)
}

func Status() (State, error) {
	dir, err := Dir()
	if err != nil {
		return State{}, err
	}
	return git.Status(dir, repoBranch)
}

func Install() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return InstallAt(dir)
}

func InstallAt(dir string) error {
	installed, err := git.Installed(dir)
	if err != nil {
		return err
	}
	if installed {
		return nil
	}
	if _, err := exec.LookPath("gh"); err == nil {
		cmd := exec.Command("gh", "repo", "clone", repoSlug, dir)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return git.Clone(repoURL, dir)
}

func InstallOrUpdate() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	installed, err := git.Installed(dir)
	if err != nil {
		return err
	}
	if !installed {
		return InstallAt(dir)
	}
	state, err := git.ComputeStatus(dir, repoBranch)
	if err != nil {
		return err
	}
	if state.Behind == 0 || state.Dirty || state.Diverged {
		return nil
	}
	return git.Pull(dir, repoBranch)
}

func Update() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	return git.Pull(dir, repoBranch)
}
