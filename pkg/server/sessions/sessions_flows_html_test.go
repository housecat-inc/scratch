package sessions

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
	"github.com/housecat-inc/scratch/testkit"
)

type sessionHarness struct {
	*testkit.HTML
	fd *fakeDeps
}

type sessionStep = testkit.Step[*sessionHarness]

var (
	sAbsent  = testkit.AbsentStep[*sessionHarness]
	sClick   = testkit.ClickStep[*sessionHarness]
	sFill    = testkit.FillStep[*sessionHarness]
	sPresent = testkit.PresentStep[*sessionHarness]
	sText    = testkit.TextContainsStep[*sessionHarness]
	sType    = testkit.TypeStep[*sessionHarness]
)

func runSessions(t *testing.T, cases []testkit.Case[*sessionHarness]) {
	testkit.RunCases(t, cases, testkit.CaseRunner[*sessionHarness]{
		Load: func(h *sessionHarness, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, tc testkit.Case[*sessionHarness]) *sessionHarness {
			fd := tc.Data.(*fakeDeps)
			s, err := NewServer(fd.deps())
			kit.R.NoError(err)
			return &sessionHarness{HTML: testkit.NewHTMLWithT(t, kit, s.Handler()), fd: fd}
		},
	})
}

func sSubmit(selector string) sessionStep {
	return func(t *testing.T, h *sessionHarness) { t.Helper(); h.Submit(selector) }
}

func sInt(label string, want int, got func(*sessionHarness) int) sessionStep {
	return func(t *testing.T, h *sessionHarness) { t.Helper(); h.A.Equal(want, got(h), label) }
}

func sBool(label string, got func(*sessionHarness) bool) sessionStep {
	return func(t *testing.T, h *sessionHarness) { t.Helper(); h.A.True(got(h), label) }
}

func sMutate(fn func(*sessionHarness)) sessionStep {
	return func(t *testing.T, h *sessionHarness) { t.Helper(); fn(h) }
}

func sHeader(name, want string) sessionStep {
	return func(t *testing.T, h *sessionHarness) { t.Helper(); h.A.Equal(want, h.Header.Get(name)) }
}

func sHomeDir(selector string) sessionStep {
	return func(t *testing.T, h *sessionHarness) {
		t.Helper()
		h.A.Contains(h.Doc.Find(selector).Text(), mustHome(t))
	}
}

func TestLoginFlowsHTML(t *testing.T) {
	runSessions(t, []testkit.Case[*sessionHarness]{
		{
			Act: []sessionStep{
				sClick(`[hx-post="/install"]`),
			},
			Assert: []sessionStep{
				sPresent("#card-install"),
				sText("body", "Update"),
				sPresent(`[hx-post="/login"]`),
			},
			Data: &fakeDeps{},
			Name: "install cascades the sign in step",
			Path: "/setup",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-post="/install"]`),
			},
			Assert: []sessionStep{
				sText("#card-install", "boom"),
				sAbsent(`[hx-post="/login"]`),
			},
			Data: &fakeDeps{installErr: errors.New("boom")},
			Name: "install error stays on the install step",
			Path: "/setup",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-post="/login"]`),
				sFill(`[name="code"]`, "abc"),
				sSubmit(`form[hx-post="/login/code"]`),
			},
			Assert: []sessionStep{
				sInt("login started once", 1, func(h *sessionHarness) int { return h.fd.loginCalls }),
				sBool("code submitted", func(h *sessionHarness) bool { return h.fd.login.submitted == "abc" }),
				sBool("login closed", func(h *sessionHarness) bool { return h.fd.login.closed }),
				sPresent("#card-login"),
				sPresent(`[hx-post="/configure"]`),
			},
			Data: &fakeDeps{installed: true, login: &fakeLogin{url: "https://claude.com/auth?code=xyz"}},
			Name: "login submits a code and cascades configure",
			Path: "/setup",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-post="/login"]`),
				sFill(`[name="code"]`, "bad"),
				sSubmit(`form[hx-post="/login/code"]`),
			},
			Assert: []sessionStep{
				sText("#card-login", "invalid code"),
				sPresent(`#card-login [href*="claude.com/auth"]`),
			},
			Data: &fakeDeps{installed: true, login: &fakeLogin{submitErr: errors.New("invalid code"), url: "https://claude.com/auth"}},
			Name: "invalid code keeps the retry form",
			Path: "/setup",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-post="/codex/login"]`),
				sMutate(func(h *sessionHarness) { h.fd.codexLogin.done = true }),
				sClick(`[hx-get="/codex/login"]`),
			},
			Assert: []sessionStep{
				sBool("codex login closed", func(h *sessionHarness) bool { return h.fd.codexLogin.closed }),
				sAbsent(`[hx-get="/codex/login"]`),
				sPresent("#card-codex-login"),
			},
			Data: &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{code: "32LO-RHYUX", url: "https://auth.openai.com/codex/device"}, installed: true},
			Name: "codex login polls until done",
			Path: "/setup",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-post="/configure"]`),
			},
			Assert: []sessionStep{
				sPresent("#card-configure"),
				sAbsent(`[hx-post="/configure"]`),
				sInt("configured once", 1, func(h *sessionHarness) int { return h.fd.configureCalls }),
				sBool("configured", func(h *sessionHarness) bool { return h.fd.configured }),
			},
			Data: &fakeDeps{authenticated: true, installed: true},
			Name: "configure marks the step done",
			Path: "/setup",
		},
	})
}

func TestSessionFlowsHTML(t *testing.T) {
	started := func() *fakeDeps {
		fd := &fakeDeps{
			authenticated: true, configured: true, installed: true,
			slugForPrompt: func(p string) string {
				if p == "" {
					return ""
				}
				return "fix-login-bug"
			},
		}
		fd.startSessionFn = wrapAppend(fd, func(name, dir, prompt string) (*claudecode.Session, error) {
			return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc"}, nil
		})
		return fd
	}

	runSessions(t, []testkit.Case[*sessionHarness]{
		{
			Act: []sessionStep{
				sFill("#session-dir", "/tmp/work"),
				sType(`[name="prompt"]`, "fix the login bug"),
				sSubmit("#session-form"),
			},
			Assert: []sessionStep{
				sInt("started once", 1, func(h *sessionHarness) int { return h.fd.startCalls }),
				sText("#card-sessions", "https://claude.ai/code/session_abc"),
				sText("#card-sessions", "fix-login-bug"),
				sText("#card-sessions", "/tmp/work"),
				sPresent(`img[src="/sessions/id1/qr"]`),
			},
			Data: started(),
			Name: "start renders the running session",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sSubmit("#session-form"),
			},
			Assert: []sessionStep{
				sHomeDir("#card-sessions"),
			},
			Data: &fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc"}, nil
				},
			},
			Name: "start defaults to the home directory",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sFill("#session-dir", "/tmp"),
				sSubmit("#session-form"),
			},
			Assert: []sessionStep{
				sText("#card-sessions", "tmux missing"),
			},
			Data: &fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return nil, errors.New("tmux missing")
				},
			},
			Name: "start surfaces errors",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sMutate(func(h *sessionHarness) {
					h.Doc.Find("#session-form").AppendHtml(`<input type="hidden" name="redirect" value="1"/>`)
				}),
				sSubmit("#session-form"),
			},
			Assert: []sessionStep{
				sHeader("HX-Redirect", "/"),
			},
			Data: started(),
			Name: "start with redirect sends the client home",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-get^="/sessions/picker"]`),
			},
			Assert: []sessionStep{
				sText("#dir-picker", "alpha"),
				sText("#dir-picker", "beta"),
				sText("#dir-picker", "Up"),
				sPresent(`#dir-picker [hx-get^="/sessions/picker?dir="]`),
			},
			Data: &fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return []string{"alpha", "beta"}, nil },
			},
			Name: "browse lists subdirectories",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-get^="/sessions/picker"]`),
			},
			Assert: []sessionStep{
				sText("#dir-picker", "No subdirectories"),
			},
			Data: &fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return nil, nil },
			},
			Name: "browse reports an empty directory",
			Path: "/",
		},
		{
			Act: []sessionStep{
				sClick(`[hx-delete="/sessions/abcd"]`),
			},
			Assert: []sessionStep{
				sInt("stopped once", 1, func(h *sessionHarness) int { return h.fd.stopCalls }),
				sText("#card-sessions", "No sessions running yet."),
			},
			Data: &fakeDeps{
				authenticated: true, configured: true, installed: true,
				sessions: []*claudecode.Session{{Dir: "/tmp", ID: "abcd", Name: "x", URL: "https://claude.ai/code/x"}},
			},
			Name: "stop removes the session",
			Path: "/",
		},
		{
			Assert: []sessionStep{
				sPresent(`a[href="/setup"]`),
				sText("body", "Finish"),
				sAbsent(`[hx-post="/sessions"]`),
			},
			Data: &fakeDeps{installed: true},
			Name: "gate points unfinished setup back to setup",
			Path: "/",
		},
		{
			Assert: []sessionStep{
				sPresent(`[hx-post="/sessions"]`),
				sPresent(`[name="prompt"]`),
				sText("body", "No sessions running yet."),
				sAbsent(`[name="name"]`),
			},
			Data: &fakeDeps{authenticated: true, installed: true},
			Name: "gate shows the form when signed in",
			Path: "/",
		},
	})
}
