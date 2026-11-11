package testspec

import (
	"slices"
	"strings"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Doc:  "check that tests are stdlib table-driven with a readable Test, group, case hierarchy",
	Name: "testspec",
	Run:  run,
}

var Pending = []string{
	"github.com/housecat-inc/scratch/testkit/v2",
}

func run(pass *analysis.Pass) (any, error) {
	if pending(pass.Pkg.Path()) {
		return nil, nil
	}
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Package).Filename
		if !strings.HasSuffix(filename, "_test.go") {
			continue
		}
		for _, p := range Parse(filename, file).Problems {
			pass.Reportf(p.Pos, "%s", p.Message)
		}
	}
	return nil, nil
}

func pending(path string) bool {
	return slices.Contains(Pending, strings.TrimSuffix(path, "_test"))
}
