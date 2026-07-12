package claudecode

import (
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

func symlinkSources() map[string]string {
	return map[string]string{
		".claude/skills": filepath.Join("scratch", "agents", "skills"),
		"AGENTS.md":      filepath.Join("scratch", "AGENTS.md"),
		"CLAUDE.md":      filepath.Join("scratch", "CLAUDE.md"),
	}
}

func symlinkTarget(home, link, src string) (string, error) {
	return filepath.Rel(filepath.Dir(filepath.Join(home, link)), filepath.Join(home, src))
}

func EnsureSymlinks() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "user home dir")
	}
	return EnsureSymlinksAt(home)
}

func EnsureSymlinksAt(home string) error {
	for name, src := range symlinkSources() {
		link := filepath.Join(home, name)
		target, err := symlinkTarget(home, name, src)
		if err != nil {
			return errors.Wrapf(err, "compute target for %s", link)
		}
		if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
			return errors.Wrapf(err, "mkdir %s", filepath.Dir(link))
		}
		fi, err := os.Lstat(link)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.Wrapf(err, "stat %s", link)
		}
		if err == nil {
			if fi.Mode()&os.ModeSymlink == 0 {
				return errors.Errorf("%s exists and is not a symlink", link)
			}
			existing, err := os.Readlink(link)
			if err != nil {
				return errors.Wrapf(err, "readlink %s", link)
			}
			if existing == target {
				continue
			}
			if err := os.Remove(link); err != nil {
				return errors.Wrapf(err, "remove stale symlink %s", link)
			}
		}
		if err := os.Symlink(target, link); err != nil {
			return errors.Wrapf(err, "symlink %s", link)
		}
	}
	return nil
}

func HasSymlinks() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.Wrap(err, "user home dir")
	}
	return HasSymlinksAt(home)
}

func HasSymlinksAt(home string) (bool, error) {
	for name, src := range symlinkSources() {
		link := filepath.Join(home, name)
		target, err := symlinkTarget(home, name, src)
		if err != nil {
			return false, errors.Wrapf(err, "compute target for %s", link)
		}
		fi, err := os.Lstat(link)
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}
		if err != nil {
			return false, errors.Wrapf(err, "stat %s", link)
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			return false, nil
		}
		existing, err := os.Readlink(link)
		if err != nil {
			return false, errors.Wrapf(err, "readlink %s", link)
		}
		if existing != target {
			return false, nil
		}
	}
	return true, nil
}
