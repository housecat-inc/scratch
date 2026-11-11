package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

type GoLiteralsIn struct {
	Offset int
	Source string
	Wrap   bool
}

const wrapPrefix = "_twsort("

// templ's multi-value class syntax -- class={ "a", cond } -- is a comma
// separated list, not a standalone Go expression, so it only parses as call
// arguments. The wrapper's width is subtracted back out of every offset.
func GoLiterals(in GoLiteralsIn) ([]Literal, error) {
	source := in.Source
	shift := in.Offset
	if in.Wrap {
		source = wrapPrefix + in.Source + ")"
		shift -= len(wrapPrefix)
	}

	fset := token.NewFileSet()
	expr, err := parser.ParseExprFrom(fset, "", source, 0)
	if err != nil {
		return nil, errors.Wrap(err, "parse go expression")
	}

	c := &goCollector{fset: fset, shift: shift}
	// Every direct argument of the synthetic wrapper is one entry of a class
	// attribute's value list, so a bare literal among them is a class string in
	// its own right; a templ.KV label is a nested call, not a literal, so it is
	// left alone. templ joins that list itself, without TwMerge, which is why
	// these entries are as safe to sort as a plain attribute.
	if wrapped, ok := expr.(*ast.CallExpr); ok && in.Wrap {
		for _, arg := range wrapped.Args {
			c.add(arg, false)
		}
	}
	c.walk(expr, "")
	return c.literals, nil
}

// GoLiteralsInDecls handles whole Go source blocks -- a .templ file's top level
// func bodies -- where returns need their enclosing func name to be judged.
func GoLiteralsInDecls(in GoLiteralsIn) ([]Literal, error) {
	const preamble = "package p\n"

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "", preamble+in.Source, 0)
	if err != nil {
		return nil, errors.Wrap(err, "parse go source")
	}

	c := &goCollector{fset: fset, shift: in.Offset - len(preamble)}
	for _, decl := range file.Decls {
		function, ok := decl.(*ast.FuncDecl)
		if !ok {
			c.walk(decl, "")
			continue
		}
		c.walk(function.Body, function.Name.Name)
	}
	return c.literals, nil
}

type goCollector struct {
	fset     *token.FileSet
	literals []Literal
	shift    int
}

func isClassName(name string) bool {
	return strings.Contains(strings.ToLower(name), "class")
}

func isMergeCall(call *ast.CallExpr) bool {
	switch fun := call.Fun.(type) {
	case *ast.SelectorExpr:
		return fun.Sel.Name == "TwMerge"
	case *ast.Ident:
		return fun.Name == "TwMerge"
	}
	return false
}

// enclosing carries the name of the func being walked so a bare `return "..."`
// can be judged by the func's own name, which is where the densest Tailwind in
// the repo lives (variantClasses, sizeClasses, getSideClasses).
func (c *goCollector) walk(node ast.Node, enclosing string) {
	ast.Inspect(node, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.CallExpr:
			if isMergeCall(v) {
				for _, arg := range v.Args {
					c.add(arg, true)
				}
			}
		case *ast.KeyValueExpr:
			if key, ok := v.Key.(*ast.Ident); ok && isClassName(key.Name) {
				c.add(v.Value, true)
			}
		case *ast.AssignStmt:
			for i, lhs := range v.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if ok && isClassName(ident.Name) && i < len(v.Rhs) {
					c.add(v.Rhs[i], true)
				}
			}
		case *ast.ValueSpec:
			for i, name := range v.Names {
				if isClassName(name.Name) && i < len(v.Values) {
					c.add(v.Values[i], true)
				}
			}
		case *ast.ReturnStmt:
			if !isClassName(enclosing) {
				return true
			}
			for _, result := range v.Results {
				c.add(result, true)
			}
		case *ast.FuncLit:
			return false
		}
		return true
	})
}

// token.Pos is 1-based, so the opening quote sits at byte Pos()-1 and the first
// class character at Pos(). End() is one past the closing quote, so the closing
// quote -- the exclusive end of the value -- sits at End()-2.
func (c *goCollector) add(expr ast.Expr, merged bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return
	}
	value, err := strconv.Unquote(lit.Value)
	if err != nil || strings.TrimSpace(value) == "" {
		return
	}

	position := c.fset.Position(lit.Pos())
	c.literals = append(c.literals, Literal{
		Column: position.Column,
		End:    c.shift + int(lit.End()) - 2,
		Line:   position.Line,
		Merged: merged,
		Start:  c.shift + int(lit.Pos()),
		Value:  value,
	})
}
