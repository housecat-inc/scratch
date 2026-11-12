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

func tree(root string) (map[string][]*Node, error) {
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

func Spec(w io.Writer, root string) error {
	files, err := tree(root)
	if err != nil {
		return err
	}
	paths := make([]string, 0, len(files))
	for path, nodes := range files {
		if len(nodes) > 0 {
			paths = append(paths, path)
		}
	}
	slices.Sort(paths)
	var b strings.Builder
	for i, path := range paths {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(path + "\n")
		writeNodes(&b, files[path], 1)
	}
	_, err = io.WriteString(w, b.String())
	return errors.Wrap(err, "write")
}

func writeNodes(b *strings.Builder, nodes []*Node, depth int) {
	nodes = slices.SortedFunc(slices.Values(nodes), func(a, b *Node) int { return cmp.Compare(a.Name, b.Name) })
	for _, n := range nodes {
		fmt.Fprintf(b, "%s%s\n", strings.Repeat("  ", depth), n.Name)
		writeNodes(b, n.Children, depth+1)
	}
}
