package testspec

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

var Analyzer = &analysis.Analyzer{
	Doc:  "check that tests are stdlib table-driven with a readable Test, group, case hierarchy",
	Name: "testspec",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		filename := pass.Fset.Position(file.Package).Filename
		if ast.IsGenerated(file) {
			continue
		}
		for _, p := range Parse(filename, file).Problems {
			pass.Reportf(p.Pos, "%s", p.Message)
		}
	}
	return nil, nil
}
