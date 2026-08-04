package testkit

import "testing"

type Step[H any] func(t *T, h H)

type Scenario[H any] struct {
	Name  string
	Setup func(t *T) H
	Steps []Step[H]
}

func (s Scenario[H]) name() string { return s.Name }

func RunSteps[H any](t *testing.T, scenarios []Scenario[H], setup func(t *T) H, opts ...Option) {
	t.Helper()
	each(t, scenarios, Scenario[H].name, func(tk *T, s Scenario[H]) {
		build := setup
		if s.Setup != nil {
			build = s.Setup
		}
		if build == nil {
			tk.Fatalf("scenario %q has no Setup and RunSteps has no default", s.Name)
		}
		h := build(tk)
		for _, step := range s.Steps {
			step(tk, h)
		}
	}, opts...)
}

type harness interface {
	Click(string)
	ElementAbsent(string)
	ElementAttributeContains(string, string, string)
	ElementAttributeEquals(string, string, string)
	ElementHidden(string)
	ElementPresent(string)
	ElementTextContains(string, string)
	ElementVisible(string)
	Fill(string, string)
	Load(string)
	SelectOption(string, string)
	Submit(string)
	Type(string, string)
}

type Steps[H harness] struct{}

func (Steps[H]) Absent(selector string) Step[H]                   { return Absent[H](selector) }
func (Steps[H]) AttrContains(selector, name, exp string) Step[H]  { return AttrContains[H](selector, name, exp) }
func (Steps[H]) AttrEquals(selector, name, exp string) Step[H]    { return AttrEquals[H](selector, name, exp) }
func (Steps[H]) Click(selector string) Step[H]                    { return Click[H](selector) }
func (Steps[H]) Fill(selector, text string) Step[H]               { return Fill[H](selector, text) }
func (Steps[H]) Hidden(selector string) Step[H]                   { return Hidden[H](selector) }
func (Steps[H]) Present(selector string) Step[H]                  { return Present[H](selector) }
func (Steps[H]) Select(selector, value string) Step[H]            { return Select[H](selector, value) }
func (Steps[H]) Submit(selector string) Step[H]                   { return Submit[H](selector) }
func (Steps[H]) Text(selector, exp string) Step[H]                { return Text[H](selector, exp) }
func (Steps[H]) Type(selector, text string) Step[H]               { return Type[H](selector, text) }
func (Steps[H]) Visible(selector string) Step[H]                  { return Visible[H](selector) }
func (Steps[H]) Visit(path string) Step[H]                        { return Visit[H](path) }

func Visit[H interface{ Load(string) }](path string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.Load(path) }
}

func Click[H interface{ Click(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.Click(selector) }
}

func Submit[H interface{ Submit(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.Submit(selector) }
}

func Fill[H interface{ Fill(string, string) }](selector, text string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.Fill(selector, text) }
}

func Type[H interface{ Type(string, string) }](selector, text string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.Type(selector, text) }
}

func Select[H interface{ SelectOption(string, string) }](selector, value string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.SelectOption(selector, value) }
}

func Present[H interface{ ElementPresent(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementPresent(selector) }
}

func Absent[H interface{ ElementAbsent(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementAbsent(selector) }
}

func Visible[H interface{ ElementVisible(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementVisible(selector) }
}

func Hidden[H interface{ ElementHidden(string) }](selector string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementHidden(selector) }
}

func Text[H interface{ ElementTextContains(string, string) }](selector, expected string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementTextContains(selector, expected) }
}

func AttrContains[H interface {
	ElementAttributeContains(string, string, string)
}](selector, name, expected string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementAttributeContains(selector, name, expected) }
}

func AttrEquals[H interface {
	ElementAttributeEquals(string, string, string)
}](selector, name, expected string) Step[H] {
	return func(t *T, h H) { t.Helper(); h.ElementAttributeEquals(selector, name, expected) }
}
