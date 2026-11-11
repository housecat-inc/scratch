package testspec

import (
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"

	"github.com/cockroachdb/errors"
)

func Lint(dir string) ([]string, error) {
	paths, err := filepath.Glob(filepath.Join(dir, "*_test.go"))
	if err != nil {
		return nil, errors.Wrapf(err, "glob %s", dir)
	}
	fset := token.NewFileSet()
	var out []string
	for _, path := range paths {
		f, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, errors.Wrapf(err, "parse %s", path)
		}
		for _, p := range Parse(path, f).Problems {
			out = append(out, fset.Position(p.Pos).String()+": "+p.Message)
		}
	}
	slices.Sort(out)
	return out, nil
}
