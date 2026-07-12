package testkit

import (
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/input"
)

var keys = map[string]input.Key{
	"Backspace": input.Backspace,
	"Ctrl":      input.ControlLeft,
	"Delete":    input.Delete,
	"Enter":     input.Enter,
	"Escape":    input.Escape,
	"Meta":      input.MetaLeft,
	"Shift":     input.ShiftLeft,
	"Tab":       input.Tab,
}

func (t *T) ElementAttributeContains(page *rod.Page, selector string, name string, expected string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		el, err := page.Element(selector)
		if err != nil {
			return false
		}

		value, err := el.Attribute(name)
		return err == nil && value != nil && strings.Contains(*value, expected)
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) ElementAttributeEquals(page *rod.Page, selector string, name string, expected string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		el, err := page.Element(selector)
		if err != nil {
			return false
		}

		value, err := el.Attribute(name)
		return err == nil && value != nil && *value == expected
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) ElementHidden(page *rod.Page, selector string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		el, err := page.Element(selector)
		if err != nil {
			return false
		}

		visible, err := el.Visible()
		return err == nil && !visible
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) ElementVisible(page *rod.Page, selector string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		el, err := page.Element(selector)
		if err != nil {
			return false
		}

		visible, err := el.Visible()
		return err == nil && visible
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) ElementTextContains(page *rod.Page, selector string, expected string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		value, err := page.Eval(`(selector) => {
			const el = document.querySelector(selector);
			return el ? el.textContent : "";
		}`, selector)
		if err != nil {
			return false
		}
		return strings.Contains(value.Value.Str(), expected)
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) Click(page *rod.Page, selector string) {
	t.t.Helper()
	page.MustElement(selector).MustClick()
}

func (t *T) ElementAbsent(page *rod.Page, selector string) {
	t.t.Helper()

	t.R.Eventually(func() bool {
		has, _, err := page.Has(selector)
		return err == nil && !has
	}, BrowserWaitTimeout, BrowserPollInterval)
}

func (t *T) Press(page *rod.Page, selector string, chord string) {
	t.t.Helper()

	parts := strings.Split(chord, "+")
	pressed := make([]input.Key, len(parts))
	for i, part := range parts {
		k, ok := keys[part]
		if !ok {
			t.t.Fatal(fmt.Errorf("unknown key %q", part))
		}
		pressed[i] = k
	}

	page.MustElement(selector).MustFocus()
	for _, k := range pressed[:len(pressed)-1] {
		if err := page.Keyboard.Press(k); err != nil {
			t.t.Fatal(err)
		}
	}
	page.Keyboard.MustType(pressed[len(pressed)-1])
	for i := len(pressed) - 2; i >= 0; i-- {
		if err := page.Keyboard.Release(pressed[i]); err != nil {
			t.t.Fatal(err)
		}
	}
}

func (t *T) Type(page *rod.Page, selector string, text string) {
	t.t.Helper()
	page.MustElement(selector).MustInput(text)
}

func (t *T) Load(page *rod.Page, baseURL string, path string) {
	t.t.Helper()
	page.MustNavigate(baseURL + path).MustWaitLoad()
}

func (t *T) Screenshot(page *rod.Page, name string) {
	t.t.Helper()
	t.Artifacts.Screenshot(page, name)
}

func (t *T) MatchScreenshot(page *rod.Page, name string) {
	t.t.Helper()
	t.Artifacts.MatchScreenshot(page, name)
}

func (t *T) Viewport(page *rod.Page, width int, height int) {
	t.t.Helper()
	viewport(page, width, height)
}
