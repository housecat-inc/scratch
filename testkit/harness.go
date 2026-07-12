package testkit

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type Harness struct {
	Browser *rod.Browser
	Console *ConsoleRecorder
	Page    *rod.Page
	Server  *httptest.Server
	*T
}

type Screenshot struct {
	Height int
	Name   string
	Width  int
}

func NewHarness(t *testing.T, handler http.Handler) *Harness {
	t.Helper()
	return NewHarnessWithT(t, New(t), handler)
}

func NewHarnessWithT(t *testing.T, kit *T, handler http.Handler) *Harness {
	t.Helper()
	server := httptest.NewServer(handler)

	controlURL := launcher.New().
		Headless(true).
		NoSandbox(true).
		MustLaunch()
	browser := rod.New().ControlURL(controlURL).MustConnect()
	page := browser.MustPage()

	t.Cleanup(func() {
		page.MustClose()
		browser.MustClose()
		server.Close()
	})

	return &Harness{
		Browser: browser,
		Console: NewConsoleRecorder(t, kit.Artifacts, page),
		Page:    page,
		Server:  server,
		T:       kit,
	}
}

func (h *Harness) Click(selector string) {
	h.T.Click(h.Page, selector)
}

func (h *Harness) ElementAbsent(selector string) {
	h.T.ElementAbsent(h.Page, selector)
}

func (h *Harness) ElementAttributeContains(selector string, name string, expected string) {
	h.T.ElementAttributeContains(h.Page, selector, name, expected)
}

func (h *Harness) ElementAttributeEquals(selector string, name string, expected string) {
	h.T.ElementAttributeEquals(h.Page, selector, name, expected)
}

func (h *Harness) ElementHidden(selector string) {
	h.T.ElementHidden(h.Page, selector)
}

func (h *Harness) ElementTextContains(selector string, expected string) {
	h.T.ElementTextContains(h.Page, selector, expected)
}

func (h *Harness) ElementVisible(selector string) {
	h.T.ElementVisible(h.Page, selector)
}

func (h *Harness) Load(path string) {
	h.T.Load(h.Page, h.Server.URL, path)
}

func (h *Harness) MatchScreenshot(name string) {
	h.T.MatchScreenshot(h.Page, name)
}

func (h *Harness) MatchScreenshots(screenshots []Screenshot, phase string, path string) {
	for _, screenshot := range screenshots {
		h.Viewport(screenshot.Width, screenshot.Height)
		if phase == "begin" {
			h.Load(path)
		}
		h.MatchScreenshot(screenshot.Name + "-" + phase + ".png")
	}
}

func (h *Harness) Press(selector string, key string) {
	h.T.Press(h.Page, selector, key)
}

func (h *Harness) Screenshot(name string) {
	h.T.Screenshot(h.Page, name)
}

func (h *Harness) Type(selector string, text string) {
	h.T.Type(h.Page, selector, text)
}

func (h *Harness) Viewport(width int, height int) {
	h.T.Viewport(h.Page, width, height)
}

func viewport(page *rod.Page, width int, height int) {
	page.MustSetViewport(width, height, 1, false)
	page.MustSetUserAgent(&proto.NetworkSetUserAgentOverride{UserAgent: "go-rod"})
}
