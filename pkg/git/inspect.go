package git

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

type Commit struct {
	Author  string
	Date    time.Time
	SHA     string
	Subject string
}

func Branch(dir string) (string, error) {
	out, err := outputIn(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

func CommitLog(dir, base, head string) ([]Commit, error) {
	if head == "" {
		head = "HEAD"
	}
	out, err := outputIn(dir, "git", "log", base+".."+head, "--format=%H%x00%an%x00%aI%x00%s")
	if err != nil {
		return nil, err
	}
	out = strings.TrimRight(out, "\n")
	if out == "" {
		return nil, nil
	}
	var commits []Commit
	for _, line := range strings.Split(out, "\n") {
		parts := strings.SplitN(line, "\x00", 4)
		if len(parts) != 4 {
			return nil, errors.Errorf("bad git log output: %q", line)
		}
		date, err := time.Parse(time.RFC3339, parts[2])
		if err != nil {
			return nil, errors.Wrapf(err, "parse date %q", parts[2])
		}
		commits = append(commits, Commit{Author: parts[1], Date: date, SHA: parts[0], Subject: parts[3]})
	}
	return commits, nil
}

func DefaultBranch(dir string) (string, error) {
	out, err := outputIn(dir, "git", "symbolic-ref", "--short", "refs/remotes/origin/HEAD")
	if err == nil {
		return strings.TrimSpace(out), nil
	}
	for _, b := range []string{"main", "master"} {
		if _, verifyErr := outputIn(dir, "git", "rev-parse", "--verify", "--quiet", b); verifyErr == nil {
			return b, nil
		}
	}
	return "", errors.New("no default branch found")
}

func LastCommit(dir string) (Commit, error) {
	out, err := outputIn(dir, "git", "log", "-1", "--format=%H%x00%an%x00%aI%x00%s")
	if err != nil {
		return Commit{}, err
	}
	parts := strings.SplitN(strings.TrimRight(out, "\n"), "\x00", 4)
	if len(parts) != 4 {
		return Commit{}, errors.Errorf("bad git log output: %q", out)
	}
	date, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return Commit{}, errors.Wrapf(err, "parse date %q", parts[2])
	}
	return Commit{Author: parts[1], Date: date, SHA: parts[0], Subject: parts[3]}, nil
}

func ShowCommit(dir, sha string) (Commit, error) {
	out, err := outputIn(dir, "git", "log", "-1", "--format=%H%x00%an%x00%aI%x00%s", sha)
	if err != nil {
		return Commit{}, err
	}
	parts := strings.SplitN(strings.TrimRight(out, "\n"), "\x00", 4)
	if len(parts) != 4 {
		return Commit{}, errors.Errorf("bad git log output: %q", out)
	}
	date, err := time.Parse(time.RFC3339, parts[2])
	if err != nil {
		return Commit{}, errors.Wrapf(err, "parse date %q", parts[2])
	}
	return Commit{Author: parts[1], Date: date, SHA: parts[0], Subject: parts[3]}, nil
}

func HunkCommit(dir, base, head, path string, newStart, newCount int) (Commit, error) {
	if path == "" || newCount <= 0 {
		return Commit{}, nil
	}
	end := newStart + newCount - 1
	if end < newStart {
		end = newStart
	}
	rangeArg := fmt.Sprintf("-L%d,%d:%s", newStart, end, path)
	out, err := outputIn(dir, "git", "log", rangeArg, "--format=%H%x00%an%x00%aI%x00%s", "-n", "1", "-s", base+".."+head)
	if err != nil {
		return Commit{}, err
	}
	first := strings.TrimSpace(out)
	if i := strings.Index(first, "\n"); i >= 0 {
		first = first[:i]
	}
	if first == "" {
		return Commit{}, nil
	}
	parts := strings.SplitN(first, "\x00", 4)
	if len(parts) != 4 {
		return Commit{}, errors.Errorf("bad log output: %q", first)
	}
	date, _ := time.Parse(time.RFC3339, parts[2])
	return Commit{Author: parts[1], Date: date, SHA: parts[0], Subject: parts[3]}, nil
}

func ShowFile(dir, ref, path string) ([]string, error) {
	out, err := outputIn(dir, "git", "show", ref+":"+path)
	if err != nil {
		return nil, err
	}
	out = strings.TrimSuffix(out, "\n")
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func ShowBlob(dir, ref, path string) ([]byte, error) {
	cmd := exec.Command("git", "show", ref+":"+path)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil, errors.Wrapf(err, "git show %s:%s", ref, path)
	}
	return out, nil
}

func RemoteSlug(dir string) (org, name string, err error) {
	out, err := outputIn(dir, "git", "remote", "get-url", "origin")
	if err != nil {
		return "", "", err
	}
	return parseSlug(strings.TrimSpace(out))
}

func parseSlug(url string) (org, name string, err error) {
	s := strings.TrimSuffix(url, ".git")
	if i := strings.Index(s, "://"); i >= 0 {
		s = s[i+3:]
		if j := strings.Index(s, "/"); j >= 0 {
			s = s[j+1:]
		} else {
			s = ""
		}
	} else if i := strings.Index(s, ":"); i >= 0 {
		s = s[i+1:]
	}
	var parts []string
	for _, p := range strings.Split(s, "/") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	if len(parts) < 2 {
		return "", "", errors.Errorf("bad remote url: %q", url)
	}
	return parts[len(parts)-2], parts[len(parts)-1], nil
}
