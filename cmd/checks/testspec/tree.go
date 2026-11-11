package testspec

import (
	"cmp"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
)

var skipDirs = []string{".git", "node_modules", "testdata", "tmp", "vendor"}

func Tree(root string) (map[string][]*Node, error) {
	fset := token.NewFileSet()
	files := map[string][]*Node{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != root && slices.Contains(skipDirs, d.Name()) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return errors.Wrapf(err, "parse %s", path)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return errors.Wrapf(err, "relative path %s", path)
		}
		files[filepath.ToSlash(rel)] = Parse(path, f).Tests
		return nil
	})
	return files, errors.Wrapf(err, "walk %s", root)
}

func Write(w io.Writer, files map[string][]*Node) error {
	paths := make([]string, 0, len(files))
	for path, nodes := range files {
		if len(nodes) > 0 {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	for i, path := range paths {
		if i > 0 {
			if _, err := fmt.Fprintln(w); err != nil {
				return errors.Wrap(err, "write")
			}
		}
		if _, err := fmt.Fprintln(w, path); err != nil {
			return errors.Wrap(err, "write")
		}
		if err := writeNodes(w, files[path], 1); err != nil {
			return err
		}
	}
	return nil
}

func writeNodes(w io.Writer, nodes []*Node, depth int) error {
	nodes = slices.SortedFunc(slices.Values(nodes), func(a, b *Node) int { return cmp.Compare(a.Name, b.Name) })
	for _, n := range nodes {
		if _, err := fmt.Fprintf(w, "%s%s\n", strings.Repeat("  ", depth), n.Name); err != nil {
			return errors.Wrap(err, "write")
		}
		if err := writeNodes(w, n.Children, depth+1); err != nil {
			return err
		}
	}
	return nil
}
