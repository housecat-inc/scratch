package testkit

import (
	"slices"
	"testing"
)

type BrowserCase[H any] struct {
	Act     []BrowserStep[H]
	Assert  []BrowserStep[H]
	Console []string
	Name    string
	Path    string
	Seed    []BrowserStep[H]
}

type BrowserCaseRunner[H any] struct {
	BeforeAct     func(H)
	ConsoleErrors func(H) []string
	Load          func(H, string)
	Setup         func(*testing.T, *T, BrowserCase[H]) H
}

type BrowserStep[H any] func(*testing.T, H)

func RunBrowserCases[H any](t *testing.T, cases []BrowserCase[H], runner BrowserCaseRunner[H]) {
	t.Helper()
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			kit := New(t)
			h := runner.Setup(t, kit, tc)

			for _, step := range tc.Seed {
				step(t, h)
			}

			runner.Load(h, tc.Path)

			if runner.BeforeAct != nil {
				runner.BeforeAct(h)
			}

			for _, step := range tc.Act {
				step(t, h)
			}
			for _, step := range tc.Assert {
				step(t, h)
			}

			if runner.ConsoleErrors == nil {
				return
			}
			for _, msg := range runner.ConsoleErrors(h) {
				if !slices.Contains(tc.Console, msg) {
					t.Errorf("unexpected console error: %s", msg)
				}
			}
		})
	}
}

func ClassContainsStep[H interface {
	ElementAttributeContains(string, string, string)
}](selector, expected string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.ElementAttributeContains(selector, "class", expected)
	}
}

func ClickStep[H interface{ Click(string) }](selector string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.Click(selector)
	}
}

func PressStep[H interface{ Press(string, string) }](selector, key string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.Press(selector, key)
	}
}

func TextContainsStep[H interface{ ElementTextContains(string, string) }](selector, expected string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.ElementTextContains(selector, expected)
	}
}

func TypeStep[H interface{ Type(string, string) }](selector, text string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.Type(selector, text)
	}
}

func VisibleStep[H interface{ ElementVisible(string) }](selector string) BrowserStep[H] {
	return func(t *testing.T, h H) {
		t.Helper()
		h.ElementVisible(selector)
	}
}
