package errorscheck

import (
	"fmt"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
)

const cockroachErrors = "github.com/cockroachdb/errors"

var Analyzer = &analysis.Analyzer{
	Doc:  "check that errors are constructed and inspected through github.com/cockroachdb/errors",
	Name: "errors",
	Run:  run,
}

func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		if ast.IsGenerated(file) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			pkg, ok := pass.TypesInfo.Uses[id].(*types.PkgName)
			if !ok {
				return true
			}
			if !banned(pkg.Imported().Path(), sel.Sel.Name) {
				return true
			}
			pass.Report(analysis.Diagnostic{
				End:     sel.End(),
				Message: fmt.Sprintf("use %s.%s instead of %s.%s", cockroachErrors, sel.Sel.Name, id.Name, sel.Sel.Name),
				Pos:     sel.Pos(),
			})
			return true
		})
	}
	return nil, nil
}

func banned(path, selector string) bool {
	switch path {
	case "errors", "github.com/pkg/errors":
		return true
	case "fmt":
		return selector == "Errorf"
	}
	return false
}
