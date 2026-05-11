package git

import (
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

type FileStatus string

const (
	StatusAdded    FileStatus = "added"
	StatusDeleted  FileStatus = "deleted"
	StatusModified FileStatus = "modified"
	StatusRenamed  FileStatus = "renamed"
)

type File struct {
	Binary  bool
	Hunks   []Hunk
	NewPath string
	OldPath string
	Status  FileStatus
}

func (f File) Path() string {
	if f.NewPath != "" {
		return f.NewPath
	}
	return f.OldPath
}

func (f File) Adds() int {
	var n int
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Kind == LineAdd {
				n++
			}
		}
	}
	return n
}

func (f File) Dels() int {
	var n int
	for _, h := range f.Hunks {
		for _, l := range h.Lines {
			if l.Kind == LineDelete {
				n++
			}
		}
	}
	return n
}

type Hunk struct {
	Header   string
	Lines    []Line
	NewCount int
	NewStart int
	OldCount int
	OldStart int
	Section  string
}

func (h Hunk) Key() string {
	sum := sha1.Sum([]byte(h.Header))
	return hex.EncodeToString(sum[:8])
}

func (h Hunk) Adds() int {
	var n int
	for _, l := range h.Lines {
		if l.Kind == LineAdd {
			n++
		}
	}
	return n
}

func (h Hunk) Dels() int {
	var n int
	for _, l := range h.Lines {
		if l.Kind == LineDelete {
			n++
		}
	}
	return n
}

type LineKind string

const (
	LineAdd     LineKind = "add"
	LineContext LineKind = "context"
	LineDelete  LineKind = "delete"
)

type Line struct {
	Content string
	Kind    LineKind
	NewLine int
	OldLine int
}

func Diff(dir, base, head string) ([]File, error) {
	if head == "" {
		head = "HEAD"
	}
	out, err := outputIn(dir, "git", "diff", "--no-color", base+"..."+head)
	if err != nil {
		return nil, err
	}
	return ParseDiff(out)
}

func DiffCommit(dir, sha string) ([]File, error) {
	out, err := outputIn(dir, "git", "diff", "--no-color", sha+"^!")
	if err != nil {
		return nil, err
	}
	return ParseDiff(out)
}

func ParseDiff(raw string) ([]File, error) {
	var files []File
	var f *File
	var oldLine, newLine int

	flush := func() {
		if f != nil {
			files = append(files, *f)
			f = nil
		}
	}

	for _, line := range strings.Split(raw, "\n") {
		switch {
		case strings.HasPrefix(line, "diff --git "):
			flush()
			f = &File{Status: StatusModified}
			old, new, ok := parseDiffGitPaths(line)
			if ok {
				f.OldPath = old
				f.NewPath = new
			}
		case f == nil:
			continue
		case strings.HasPrefix(line, "--- "):
			old := strings.TrimPrefix(line, "--- ")
			if old == "/dev/null" {
				f.OldPath = ""
				f.Status = StatusAdded
			} else {
				f.OldPath = strings.TrimPrefix(old, "a/")
			}
		case strings.HasPrefix(line, "+++ "):
			n := strings.TrimPrefix(line, "+++ ")
			if n == "/dev/null" {
				f.NewPath = ""
				f.Status = StatusDeleted
			} else {
				f.NewPath = strings.TrimPrefix(n, "b/")
			}
		case strings.HasPrefix(line, "new file mode "):
			f.Status = StatusAdded
			f.OldPath = ""
		case strings.HasPrefix(line, "deleted file mode "):
			f.Status = StatusDeleted
			f.NewPath = ""
		case strings.HasPrefix(line, "rename from "):
			f.OldPath = strings.TrimPrefix(line, "rename from ")
			f.Status = StatusRenamed
		case strings.HasPrefix(line, "rename to "):
			f.NewPath = strings.TrimPrefix(line, "rename to ")
			f.Status = StatusRenamed
		case strings.HasPrefix(line, "Binary files "):
			f.Binary = true
		case strings.HasPrefix(line, "@@"):
			hunk, err := parseHunkHeader(line)
			if err != nil {
				return nil, err
			}
			f.Hunks = append(f.Hunks, hunk)
			oldLine = hunk.OldStart
			newLine = hunk.NewStart
		case len(f.Hunks) > 0 && len(line) > 0:
			h := &f.Hunks[len(f.Hunks)-1]
			switch line[0] {
			case '+':
				h.Lines = append(h.Lines, Line{Content: line[1:], Kind: LineAdd, NewLine: newLine})
				newLine++
			case '-':
				h.Lines = append(h.Lines, Line{Content: line[1:], Kind: LineDelete, OldLine: oldLine})
				oldLine++
			case ' ':
				h.Lines = append(h.Lines, Line{Content: line[1:], Kind: LineContext, NewLine: newLine, OldLine: oldLine})
				newLine++
				oldLine++
			}
		}
	}
	flush()
	return files, nil
}

func parseDiffGitPaths(line string) (old, new string, ok bool) {
	rest := strings.TrimPrefix(line, "diff --git ")
	if rest == line {
		return "", "", false
	}
	sep := strings.LastIndex(rest, " b/")
	if sep <= 0 {
		return "", "", false
	}
	left := rest[:sep]
	right := rest[sep+len(" b/"):]
	if !strings.HasPrefix(left, "a/") {
		return "", "", false
	}
	return strings.TrimPrefix(left, "a/"), right, true
}

func parseHunkHeader(line string) (Hunk, error) {
	h := Hunk{Header: line}
	if !strings.HasPrefix(line, "@@ ") {
		return h, errors.Errorf("bad hunk header: %q", line)
	}
	rest := strings.TrimPrefix(line, "@@ ")
	end := strings.Index(rest, " @@")
	if end < 0 {
		return h, errors.Errorf("bad hunk header: %q", line)
	}
	parts := strings.Fields(rest[:end])
	if len(parts) != 2 || !strings.HasPrefix(parts[0], "-") || !strings.HasPrefix(parts[1], "+") {
		return h, errors.Errorf("bad hunk header: %q", line)
	}
	oldStart, oldCount, err := parseHunkRange(strings.TrimPrefix(parts[0], "-"))
	if err != nil {
		return h, errors.Wrapf(err, "old range in %q", line)
	}
	newStart, newCount, err := parseHunkRange(strings.TrimPrefix(parts[1], "+"))
	if err != nil {
		return h, errors.Wrapf(err, "new range in %q", line)
	}
	h.OldStart, h.OldCount = oldStart, oldCount
	h.NewStart, h.NewCount = newStart, newCount
	if end+3 < len(rest) {
		h.Section = strings.TrimSpace(rest[end+3:])
	}
	return h, nil
}

func parseHunkRange(s string) (start, count int, err error) {
	count = 1
	if i := strings.Index(s, ","); i >= 0 {
		start, err = strconv.Atoi(s[:i])
		if err != nil {
			return 0, 0, err
		}
		count, err = strconv.Atoi(s[i+1:])
		return start, count, err
	}
	start, err = strconv.Atoi(s)
	return start, count, err
}
