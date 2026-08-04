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
		h := build(tk)
		for _, step := range s.Steps {
			step(tk, h)
		}
	}, opts...)
}

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
