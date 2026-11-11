package main

import (
	"sort"
	"strings"

	parser "github.com/a-h/templ/parser/v2"
	"github.com/cockroachdb/errors"
)

type Literal struct {
	Column int
	End    int
	Line   int
	// Merged marks a literal that reaches TwMerge, where reordering can
	// change which class survives. A plain class attribute never does: the
	// browser resolves conflicts by stylesheet order, which attribute order
	// cannot affect, so sorting one is always safe.
	Merged bool
	Start  int
	Value  string
}

type RewriteIn struct {
	CSSEntry string
	File     string
	Source   string
}

func Rewrite(in RewriteIn) (string, []Skip, error) {
	literals, err := Literals(in.File, in.Source)
	if err != nil {
		return "", nil, errors.Wrap(err, "collect literals")
	}
	if len(literals) == 0 {
		return in.Source, nil, nil
	}

	vocabulary := map[string]bool{}
	for _, literal := range literals {
		for _, class := range strings.Fields(literal.Value) {
			vocabulary[class] = true
		}
	}
	classes := make([]string, 0, len(vocabulary))
	for class := range vocabulary {
		classes = append(classes, class)
	}
	sort.Strings(classes)

	ranks, err := Ranks(RanksIn{Classes: classes, CSSEntry: in.CSSEntry})
	if err != nil {
		return "", nil, errors.Wrap(err, "rank classes")
	}

	var skips []Skip
	replacements := map[int]string{}
	for _, literal := range literals {
		sorted := strings.Join(Sort(strings.Fields(literal.Value), ranks), " ")
		if sorted == literal.Value {
			continue
		}
		if safety := Check(literal.Value, sorted); literal.Merged && !safety.OK {
			skips = append(skips, Skip{
				Column:  literal.Column,
				File:    in.File,
				Line:    literal.Line,
				Literal: literal.Value,
				Safety:  safety,
				Sorted:  sorted,
			})
			continue
		}
		replacements[literal.Start] = sorted
	}

	return Apply(in.Source, replacements, literals), skips, nil
}

func Apply(source string, replacements map[int]string, literals []Literal) string {
	var b strings.Builder
	cursor := 0
	for _, literal := range literals {
		replacement, ok := replacements[literal.Start]
		if !ok {
			continue
		}
		b.WriteString(source[cursor:literal.Start])
		b.WriteString(replacement)
		cursor = literal.End
	}
	b.WriteString(source[cursor:])
	return b.String()
}

func Literals(filename, source string) ([]Literal, error) {
	file, err := parser.ParseString(source)
	if err != nil {
		return nil, errors.Wrapf(err, "parse %s", filename)
	}

	c := &collector{}
	for _, node := range file.Nodes {
		switch n := node.(type) {
		case *parser.HTMLTemplate:
			c.nodes(n.Children)
		case *parser.TemplateFileGoExpression:
			c.goSource(n.Expression, false)
		}
	}

	sort.Slice(c.literals, func(i, j int) bool { return c.literals[i].Start < c.literals[j].Start })

	// Positions are derived from the file offset rather than from each
	// producer, because the Go fragments are parsed in their own FileSet and
	// would otherwise report lines relative to the fragment.
	for i, literal := range c.literals {
		c.literals[i].Line, c.literals[i].Column = lineColumn(source, literal.Start)
	}
	return c.literals, nil
}

func lineColumn(source string, offset int) (int, int) {
	if offset > len(source) {
		offset = len(source)
	}
	before := source[:offset]
	line := strings.Count(before, "\n") + 1
	column := offset - (strings.LastIndex(before, "\n") + 1) + 1
	return line, column
}

type collector struct {
	literals []Literal
}

// ScriptElement and RawElement contribute their attributes but never their
// contents, which is what keeps class strings embedded in JavaScript out of
// reach by construction rather than by an exclusion rule.
func (c *collector) nodes(nodes []parser.Node) {
	for _, node := range nodes {
		switch n := node.(type) {
		case *parser.Element:
			c.attributes(n.Attributes)
			c.nodes(n.Children)
		case *parser.ScriptElement:
			c.attributes(n.Attributes)
		case *parser.RawElement:
			c.attributes(n.Attributes)
		case *parser.IfExpression:
			c.nodes(n.Then)
			for _, elseIf := range n.ElseIfs {
				c.nodes(elseIf.Then)
			}
			c.nodes(n.Else)
		case *parser.ForExpression:
			c.nodes(n.Children)
		case *parser.SwitchExpression:
			for _, cse := range n.Cases {
				c.nodes(cse.Children)
			}
		case *parser.TemplElementExpression:
			// The call itself carries component props, so @Button(Props{Class:
			// "..."}) is only reachable through its expression, not its children.
			c.goSource(n.Expression, true)
			c.nodes(n.Children)
		case *parser.GoCode:
			c.goSource(n.Expression, true)
		}
	}
}

// templ accepts Go fragments that go/parser alone rejects, and a class string
// left unsorted beats refusing to format the file, so a parse failure here is
// skipped rather than fatal.
func (c *collector) goSource(expression parser.Expression, wrap bool) {
	in := GoLiteralsIn{
		Offset: int(expression.Range.From.Index),
		Source: expression.Value,
		Wrap:   wrap,
	}

	var literals []Literal
	var err error
	if wrap {
		literals, err = GoLiterals(in)
	} else {
		literals, err = GoLiteralsInDecls(in)
	}
	if err != nil {
		return
	}
	c.literals = append(c.literals, literals...)
}

func (c *collector) attributes(attributes []parser.Attribute) {
	for _, attribute := range attributes {
		switch a := attribute.(type) {
		case *parser.ConstantAttribute:
			if !isClassKey(a.Key) {
				continue
			}
			c.add(a.Value, a.ValueRange)
		case *parser.ExpressionAttribute:
			if !isClassKey(a.Key) {
				continue
			}
			c.goSource(a.Expression, true)
		}
	}
}

func isClassKey(key parser.AttributeKey) bool {
	constant, ok := key.(parser.ConstantAttributeKey)
	return ok && constant.Name == "class"
}

func (c *collector) add(value string, r parser.Range) {
	c.literals = append(c.literals, Literal{
		End:   int(r.To.Index),
		Start: int(r.From.Index),
		Value: value,
	})
}
