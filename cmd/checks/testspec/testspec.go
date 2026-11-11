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
	"github.com/housecat-inc/scratch/pkg/agents",
	"github.com/housecat-inc/scratch/pkg/api",
	"github.com/housecat-inc/scratch/pkg/chat",
	"github.com/housecat-inc/scratch/pkg/db",
	"github.com/housecat-inc/scratch/pkg/elicit",
	"github.com/housecat-inc/scratch/pkg/flow",
	"github.com/housecat-inc/scratch/pkg/git",
	"github.com/housecat-inc/scratch/pkg/harness/claudecode",
	"github.com/housecat-inc/scratch/pkg/harness/codexcode",
	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver",
	"github.com/housecat-inc/scratch/pkg/inbox",
	"github.com/housecat-inc/scratch/pkg/repo",
	"github.com/housecat-inc/scratch/pkg/server/code",
	"github.com/housecat-inc/scratch/pkg/server/sessions",
	"github.com/housecat-inc/scratch/pkg/server/sql",
	"github.com/housecat-inc/scratch/pkg/todo",
	"github.com/housecat-inc/scratch/pkg/ui",
	"github.com/housecat-inc/scratch/pkg/workflow",
	"github.com/housecat-inc/scratch/pkg/workspace",
	"github.com/housecat-inc/scratch/testkit",
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
