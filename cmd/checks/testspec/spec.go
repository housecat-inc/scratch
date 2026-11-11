package testspec

import (
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

const maxDepth = 2

var (
	identName = regexp.MustCompile(`^[A-Z][A-Za-z0-9]*$`)
	titleWord = regexp.MustCompile(`^[A-Z][a-z]`)
)

type File struct {
	Problems []Problem
	Tests    []*Node
}

type Node struct {
	Children []*Node
	Name     string
	Pos      token.Pos
}

type Problem struct {
	Message string
	Pos     token.Pos
}

type walker struct {
	problems []Problem
}

type tables map[string]*ast.CompositeLit

func Parse(filename string, f *ast.File) File {
	p := &walker{}
	isTestFile := strings.HasSuffix(filename, "_test.go")

	var tests []*ast.FuncDecl
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		if _, ok := testingParam(fn.Type); ok && isTestFile && fn.Recv == nil && isTestName(fn.Name.Name) {
			tests = append(tests, fn)
			continue
		}
		p.forbidRuns(fn.Body)
	}

	subject := strings.TrimSuffix(filepath.Base(filename), "_test.go")
	var nodes []*Node
	for i, fn := range tests {
		if i > 0 {
			p.report(fn.Name.Pos(), "one Test per file: %s already declares %s", filepath.Base(filename), tests[0].Name.Name)
		}
		if normalize(fn.Name.Name[len("Test"):]) != normalize(subject) {
			p.report(fn.Name.Pos(), "%s must be named after its file %s, like Test%s", fn.Name.Name, filepath.Base(filename), camel(subject))
		}
		t, _ := testingParam(fn.Type)
		children := p.walk(fn.Body, t, 1, tables{})
		if len(children) == 0 {
			p.report(fn.Name.Pos(), "%s must describe behavior with t.Run cases", fn.Name.Name)
		}
		nodes = append(nodes, &Node{Children: children, Name: fn.Name.Name[len("Test"):], Pos: fn.Name.Pos()})
	}
	return File{Problems: p.problems, Tests: nodes}
}

func (p *walker) forbidRuns(body *ast.BlockStmt) {
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || !isRun(call) {
			return true
		}
		if fn, ok := call.Args[1].(*ast.FuncLit); ok {
			if _, ok := testingParam(fn.Type); ok {
				p.report(call.Pos(), "t.Run must be called inside a _test.go file's Test function, not a helper")
				return false
			}
		}
		return true
	})
}

func (p *walker) walk(body *ast.BlockStmt, t string, depth int, parent tables) []*Node {
	scope := collectTables(body, parent)
	var nodes []*Node
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		if call, ok := n.(*ast.CallExpr); ok && isRunOn(call, t) {
			nodes = append(nodes, p.run(call, stack, scope, depth)...)
			return false
		}
		stack = append(stack, n)
		return true
	})

	seen := map[string]bool{}
	for _, n := range nodes {
		if seen[n.Name] {
			p.report(n.Pos, "duplicate test name %q", n.Name)
		}
		seen[n.Name] = true
	}
	return nodes
}

func (p *walker) run(call *ast.CallExpr, stack []ast.Node, scope tables, depth int) []*Node {
	fn, ok := call.Args[1].(*ast.FuncLit)
	inner, isTest := "", false
	if ok {
		inner, isTest = testingParam(fn.Type)
	}
	if !isTest {
		p.report(call.Args[1].Pos(), "t.Run body must be a func(t *testing.T) literal")
		return nil
	}
	if depth > maxDepth {
		p.report(call.Pos(), "tests nest at most three levels: Test, group, case")
	}

	children := p.walk(fn.Body, inner, depth+1, scope)
	var nodes []*Node
	for _, n := range p.names(call.Args[0], stack, scope) {
		p.checkName(n)
		nodes = append(nodes, &Node{Children: children, Name: n.Name, Pos: n.Pos})
	}
	return nodes
}

func (p *walker) names(expr ast.Expr, stack []ast.Node, scope tables) []*Node {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if s, err := strconv.Unquote(e.Value); err == nil && e.Kind == token.STRING {
			return []*Node{{Name: s, Pos: e.Pos()}}
		}
	case *ast.Ident:
		if rng := enclosingRange(stack, e.Name, true); rng != nil {
			return p.mapKeys(resolve(rng.X, scope), expr)
		}
	case *ast.SelectorExpr:
		if id, ok := e.X.(*ast.Ident); ok && e.Sel.Name == "name" {
			if rng := enclosingRange(stack, id.Name, false); rng != nil {
				return p.nameFields(resolve(rng.X, scope), expr)
			}
		}
	}
	p.report(expr.Pos(), "t.Run name must be a string literal or tt.name from a literal table")
	return nil
}

func (p *walker) mapKeys(lit *ast.CompositeLit, at ast.Expr) []*Node {
	if lit == nil {
		p.report(at.Pos(), "t.Run name must range over a literal table")
		return nil
	}
	var nodes []*Node
	for _, elt := range lit.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if n := stringLit(kv.Key); n != nil {
				nodes = append(nodes, n)
				continue
			}
		}
		p.report(elt.Pos(), "table case key must be a string literal")
	}
	return nodes
}

func (p *walker) nameFields(lit *ast.CompositeLit, at ast.Expr) []*Node {
	if lit == nil {
		p.report(at.Pos(), "t.Run name must range over a literal table")
		return nil
	}
	var nodes []*Node
	for _, elt := range lit.Elts {
		if n := nameField(elt); n != nil {
			nodes = append(nodes, n)
			continue
		}
		p.report(elt.Pos(), "table case needs a string literal name field")
	}
	return nodes
}

func (p *walker) checkName(n *Node) {
	switch {
	case n.Name == "":
		p.report(n.Pos, "test name must not be empty")
	case strings.TrimSpace(n.Name) != n.Name, strings.Contains(n.Name, "  "), strings.ContainsAny(n.Name, "_/\t\n"):
		p.report(n.Pos, "test name %q must read as words: no underscores, slashes or extra spaces", n.Name)
	case identName.MatchString(n.Name):
	case titleWord.MatchString(n.Name):
		p.report(n.Pos, "test name %q must read like a sentence starting lowercase", n.Name)
	}
}

func (p *walker) report(pos token.Pos, format string, args ...any) {
	p.problems = append(p.problems, Problem{Message: fmt.Sprintf(format, args...), Pos: pos})
}

func camel(s string) string {
	var b strings.Builder
	for _, part := range strings.FieldsFunc(s, func(r rune) bool { return r == '_' || r == '-' }) {
		b.WriteString(strings.ToUpper(part[:1]) + part[1:])
	}
	return b.String()
}

func collectTables(body *ast.BlockStmt, parent tables) tables {
	scope := tables{}
	for k, v := range parent {
		scope[k] = v
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.AssignStmt:
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || i >= len(s.Rhs) {
					continue
				}
				if lit, ok := s.Rhs[i].(*ast.CompositeLit); ok {
					scope[id.Name] = lit
				}
			}
		case *ast.ValueSpec:
			for i, id := range s.Names {
				if i >= len(s.Values) {
					continue
				}
				if lit, ok := s.Values[i].(*ast.CompositeLit); ok {
					scope[id.Name] = lit
				}
			}
		}
		return true
	})
	return scope
}

func enclosingRange(stack []ast.Node, name string, key bool) *ast.RangeStmt {
	for i := len(stack) - 1; i >= 0; i-- {
		rng, ok := stack[i].(*ast.RangeStmt)
		if !ok {
			continue
		}
		v := rng.Value
		if key {
			v = rng.Key
		}
		if id, ok := v.(*ast.Ident); ok && id.Name == name {
			return rng
		}
	}
	return nil
}

func isRun(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Run" && len(call.Args) == 2
}

func isRunOn(call *ast.CallExpr, t string) bool {
	if !isRun(call) {
		return false
	}
	id, ok := call.Fun.(*ast.SelectorExpr).X.(*ast.Ident)
	return ok && id.Name == t
}

func isTestName(name string) bool {
	rest, ok := strings.CutPrefix(name, "Test")
	if !ok {
		return false
	}
	return rest == "" || !unicode.IsLower([]rune(rest)[0])
}

func nameField(elt ast.Expr) *Node {
	lit, ok := elt.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "name" {
			return stringLit(kv.Value)
		}
	}
	return nil
}

func normalize(s string) string {
	return strings.ToLower(strings.NewReplacer("_", "", "-", "").Replace(s))
}

func resolve(x ast.Expr, scope tables) *ast.CompositeLit {
	switch e := x.(type) {
	case *ast.CompositeLit:
		return e
	case *ast.Ident:
		return scope[e.Name]
	}
	return nil
}

func stringLit(x ast.Expr) *Node {
	lit, ok := x.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return nil
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil {
		return nil
	}
	return &Node{Name: s, Pos: lit.Pos()}
}

func testingParam(ft *ast.FuncType) (string, bool) {
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) != 1 {
		return "", false
	}
	star, ok := ft.Params.List[0].Type.(*ast.StarExpr)
	if !ok {
		return "", false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "testing" || sel.Sel.Name != "T" {
		return "", false
	}
	return ft.Params.List[0].Names[0].Name, true
}
