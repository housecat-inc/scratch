package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
)

var FetchTTL = 5 * time.Minute

type State struct {
	Behind    int
	Diverged  bool
	Dirty     bool
	Installed bool
}

func Installed(dir string) (bool, error) {
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return true, nil
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, errors.Wrapf(err, "stat %s", dir)
	}
	if _, err := outputIn(dir, "git", "rev-parse", "--is-inside-work-tree"); err != nil {
		return false, nil
	}
	return true, nil
}

func Clone(url, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return errors.Wrapf(err, "mkdir %s", filepath.Dir(dir))
	}
	return run("git", "clone", url, dir)
}

func Pull(dir, branch string) error {
	return runIn(dir, "git", "pull", "--ff-only", "origin", branch)
}

func Status(dir, branch string) (State, error) {
	installed, err := Installed(dir)
	if err != nil || !installed {
		return State{Installed: installed}, err
	}
	if err := defaultFetcher.fetch(dir, branch); err != nil {
		return State{Installed: true}, err
	}
	return ComputeStatus(dir, branch)
}

func ComputeStatus(dir, branch string) (State, error) {
	state := State{Installed: true}

	porcelain, err := outputIn(dir, "git", "status", "--porcelain")
	if err != nil {
		return state, err
	}
	state.Dirty = strings.TrimSpace(porcelain) != ""

	revList, err := outputIn(dir, "git", "rev-list", "--left-right", "--count", "HEAD...origin/"+branch)
	if err != nil {
		return state, err
	}
	parts := strings.Fields(revList)
	if len(parts) != 2 {
		return state, errors.Errorf("unexpected rev-list output: %q", revList)
	}
	ahead, err := strconv.Atoi(parts[0])
	if err != nil {
		return state, errors.Wrapf(err, "parse ahead count: %q", parts[0])
	}
	behind, err := strconv.Atoi(parts[1])
	if err != nil {
		return state, errors.Wrapf(err, "parse behind count: %q", parts[1])
	}
	state.Behind = behind
	state.Diverged = ahead > 0
	return state, nil
}

type fetcher struct {
	last map[string]time.Time
	mu   sync.Mutex
}

var defaultFetcher = &fetcher{last: map[string]time.Time{}}

func (f *fetcher) fetch(dir, branch string) error {
	key := dir + "\x00" + branch
	f.mu.Lock()
	last, ok := f.last[key]
	f.mu.Unlock()
	if ok && time.Since(last) < FetchTTL {
		return nil
	}
	if err := runIn(dir, "git", "fetch", "--quiet", "origin", branch); err != nil {
		return err
	}
	f.mu.Lock()
	f.last[key] = time.Now()
	f.mu.Unlock()
	return nil
}

func outputIn(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrapf(err, "run %s", name)
	}
	return string(out), nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "run %s", name)
	}
	return nil
}

func runIn(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "run %s", name)
	}
	return nil
}
