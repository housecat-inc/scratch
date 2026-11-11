package toolbar

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/net/html"
)

// focusableTags is what can hold the roving tabindex, matching FOCUSABLE in
// toolbar.ts. An element with tabindex counts whatever it is called, which is
// the one case a tag name cannot answer.
var focusableTags = map[string]bool{
	"a":        true,
	"button":   true,
	"input":    true,
	"select":   true,
	"summary":  true,
	"textarea": true,
}

// Unwrapped names every control inside a toolbar that no Item wraps.
//
// The script walks items, so an unwrapped control is not in the toolbar at
// all: it keeps a tab stop of its own, which is exactly the thing the component
// exists to collapse, and nothing about the rendered page looks wrong. A test
// asserting this is empty is how a screen keeps its toolbar whole while people
// add actions to it.
//
// Controls inside a popover are skipped. A popover is its own widget with its
// own keys, and its contents sit inside the trigger's item in the DOM without
// ever belonging to the strip.
func Unwrapped(markup string) ([]string, error) {
	doc, err := html.Parse(strings.NewReader(markup))
	if err != nil {
		return nil, errors.Wrap(err, "parse markup")
	}
	var found []string
	var walk func(n *html.Node, inStrip, inItem, inPopover bool)
	walk = func(n *html.Node, inStrip, inItem, inPopover bool) {
		if n.Type == html.ElementNode {
			inStrip = inStrip || hasAttr(n, "data-toolbar")
			inItem = inItem || hasAttr(n, "data-toolbar-item")
			inPopover = inPopover || hasAttr(n, "popover")
			if inStrip && !inItem && !inPopover && focusable(n) {
				found = append(found, describe(n))
			}
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child, inStrip, inItem, inPopover)
		}
	}
	walk(doc, false, false, false)
	sort.Strings(found)
	return found, nil
}

// An anchor is focusable only with an href, the way the selector says: an <a>
// without one is a placeholder, and giving it a tabindex would put a stop in
// the strip that does nothing when pressed.
func focusable(n *html.Node) bool {
	if hasAttr(n, "tabindex") {
		return true
	}
	if n.Data == "a" {
		return hasAttr(n, "href")
	}
	return focusableTags[n.Data]
}

// describe says which control it is in the words its call site used, so a
// failure names the thing to wrap rather than its position in a string.
func describe(n *html.Node) string {
	out := n.Data
	for _, name := range []string{"aria-label", "data-editor-cmd", "data-slot", "class"} {
		if value := attr(n, name); value != "" {
			return fmt.Sprintf("%s[%s=%q]", out, name, value)
		}
	}
	return out
}

func attr(n *html.Node, name string) string {
	for _, a := range n.Attr {
		if a.Key == name {
			return a.Val
		}
	}
	return ""
}

func hasAttr(n *html.Node, name string) bool {
	for _, a := range n.Attr {
		if a.Key == name {
			return true
		}
	}
	return false
}
