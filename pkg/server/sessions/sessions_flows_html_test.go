package sessions

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
	"github.com/housecat-inc/scratch/testkit"
	tk "github.com/housecat-inc/scratch/testkit/v2"
)

type sessionHarness struct {
	*testkit.HTML
	fd *fakeDeps
}

var ss = tk.Steps[*sessionHarness]{}

func sessionSetup(fd *fakeDeps) func(*tk.T) *sessionHarness {
	return func(t *tk.T) *sessionHarness {
		s, err := NewServer(fd.deps())
		t.R.NoError(err)
		return &sessionHarness{HTML: testkit.NewHTML(t.T, s.Handler()), fd: fd}
	}
}

func runSessions(t *testing.T, scenarios []tk.Scenario[*sessionHarness]) {
	t.Helper()
	tk.RunSteps(t, scenarios, nil)
}

func sInt(label string, want int, got func(*sessionHarness) int) tk.Step[*sessionHarness] {
	return func(t *tk.T, h *sessionHarness) { t.Helper(); t.A.Equal(want, got(h), label) }
}

func sBool(label string, got func(*sessionHarness) bool) tk.Step[*sessionHarness] {
	return func(t *tk.T, h *sessionHarness) { t.Helper(); t.A.True(got(h), label) }
}

func sMutate(fn func(*sessionHarness)) tk.Step[*sessionHarness] {
	return func(t *tk.T, h *sessionHarness) { t.Helper(); fn(h) }
}

func sHeader(name, want string) tk.Step[*sessionHarness] {
	return func(t *tk.T, h *sessionHarness) { t.Helper(); t.A.Equal(want, h.Header.Get(name)) }
}

func sHomeDir(selector string) tk.Step[*sessionHarness] {
	return func(t *tk.T, h *sessionHarness) {
		t.Helper()
		t.A.Contains(h.Doc.Find(selector).Text(), mustHome(t.T))
	}
}

func TestLoginFlowsHTML(t *testing.T) {
	runSessions(t, []tk.Scenario[*sessionHarness]{
		{
			Name:  "install cascades the sign in step",
			Setup: sessionSetup(&fakeDeps{}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/install"]`),
				ss.Present("#card-install"),
				ss.Text("body", "Update"),
				ss.Present(`[hx-post="/login"]`),
			},
		},
		{
			Name:  "install error stays on the install step",
			Setup: sessionSetup(&fakeDeps{installErr: errors.New("boom")}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/install"]`),
				ss.Text("#card-install", "boom"),
				ss.Absent(`[hx-post="/login"]`),
			},
		},
		{
			Name:  "login submits a code and cascades configure",
			Setup: sessionSetup(&fakeDeps{installed: true, login: &fakeLogin{url: "https://claude.com/auth?code=xyz"}}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/login"]`),
				ss.Fill(`[name="code"]`, "abc"),
				ss.Submit(`form[hx-post="/login/code"]`),
				sInt("login started once", 1, func(h *sessionHarness) int { return h.fd.loginCalls }),
				sBool("code submitted", func(h *sessionHarness) bool { return h.fd.login.submitted == "abc" }),
				sBool("login closed", func(h *sessionHarness) bool { return h.fd.login.closed }),
				ss.Present("#card-login"),
				ss.Present(`[hx-post="/configure"]`),
			},
		},
		{
			Name:  "invalid code keeps the retry form",
			Setup: sessionSetup(&fakeDeps{installed: true, login: &fakeLogin{submitErr: errors.New("invalid code"), url: "https://claude.com/auth"}}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/login"]`),
				ss.Fill(`[name="code"]`, "bad"),
				ss.Submit(`form[hx-post="/login/code"]`),
				ss.Text("#card-login", "invalid code"),
				ss.Present(`#card-login [href*="claude.com/auth"]`),
			},
		},
		{
			Name:  "codex login polls until done",
			Setup: sessionSetup(&fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{code: "32LO-RHYUX", url: "https://auth.openai.com/codex/device"}, installed: true}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/codex/login"]`),
				sMutate(func(h *sessionHarness) { h.fd.codexLogin.done = true }),
				ss.Click(`[hx-get="/codex/login"]`),
				sBool("codex login closed", func(h *sessionHarness) bool { return h.fd.codexLogin.closed }),
				ss.Absent(`[hx-get="/codex/login"]`),
				ss.Present("#card-codex-login"),
			},
		},
		{
			Name:  "configure marks the step done",
			Setup: sessionSetup(&fakeDeps{authenticated: true, installed: true}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/setup"),
				ss.Click(`[hx-post="/configure"]`),
				ss.Present("#card-configure"),
				ss.Absent(`[hx-post="/configure"]`),
				sInt("configured once", 1, func(h *sessionHarness) int { return h.fd.configureCalls }),
				sBool("configured", func(h *sessionHarness) bool { return h.fd.configured }),
			},
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

	runSessions(t, []tk.Scenario[*sessionHarness]{
		{
			Name:  "start renders the running session",
			Setup: sessionSetup(started()),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Fill("#session-dir", "/tmp/work"),
				ss.Type(`[name="prompt"]`, "fix the login bug"),
				ss.Submit("#session-form"),
				sInt("started once", 1, func(h *sessionHarness) int { return h.fd.startCalls }),
				ss.Text("#card-sessions", "https://claude.ai/code/session_abc"),
				ss.Text("#card-sessions", "fix-login-bug"),
				ss.Text("#card-sessions", "/tmp/work"),
				ss.Present(`img[src="/sessions/id1/qr"]`),
			},
		},
		{
			Name: "start defaults to the home directory",
			Setup: sessionSetup(&fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc"}, nil
				},
			}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Submit("#session-form"),
				sHomeDir("#card-sessions"),
			},
		},
		{
			Name: "start surfaces errors",
			Setup: sessionSetup(&fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return nil, errors.New("tmux missing")
				},
			}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Fill("#session-dir", "/tmp"),
				ss.Submit("#session-form"),
				ss.Text("#card-sessions", "tmux missing"),
			},
		},
		{
			Name:  "start with redirect sends the client home",
			Setup: sessionSetup(started()),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				sMutate(func(h *sessionHarness) {
					h.Doc.Find("#session-form").AppendHtml(`<input type="hidden" name="redirect" value="1"/>`)
				}),
				ss.Submit("#session-form"),
				sHeader("HX-Redirect", "/"),
			},
		},
		{
			Name: "browse lists subdirectories",
			Setup: sessionSetup(&fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return []string{"alpha", "beta"}, nil },
			}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Click(`[hx-get^="/sessions/picker"]`),
				ss.Text("#dir-picker", "alpha"),
				ss.Text("#dir-picker", "beta"),
				ss.Text("#dir-picker", "Up"),
				ss.Present(`#dir-picker [hx-get^="/sessions/picker?dir="]`),
			},
		},
		{
			Name: "browse reports an empty directory",
			Setup: sessionSetup(&fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return nil, nil },
			}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Click(`[hx-get^="/sessions/picker"]`),
				ss.Text("#dir-picker", "No subdirectories"),
			},
		},
		{
			Name: "stop removes the session",
			Setup: sessionSetup(&fakeDeps{
				authenticated: true, configured: true, installed: true,
				sessions: []*claudecode.Session{{Dir: "/tmp", ID: "abcd", Name: "x", URL: "https://claude.ai/code/x"}},
			}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Click(`[hx-delete="/sessions/abcd"]`),
				sInt("stopped once", 1, func(h *sessionHarness) int { return h.fd.stopCalls }),
				ss.Text("#card-sessions", "No sessions running yet."),
			},
		},
		{
			Name:  "gate points unfinished setup back to setup",
			Setup: sessionSetup(&fakeDeps{installed: true}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Present(`a[href="/setup"]`),
				ss.Text("body", "Finish"),
				ss.Absent(`[hx-post="/sessions"]`),
			},
		},
		{
			Name:  "gate shows the form when signed in",
			Setup: sessionSetup(&fakeDeps{authenticated: true, installed: true}),
			Steps: []tk.Step[*sessionHarness]{
				ss.Visit("/"),
				ss.Present(`[hx-post="/sessions"]`),
				ss.Present(`[name="prompt"]`),
				ss.Text("body", "No sessions running yet."),
				ss.Absent(`[name="name"]`),
			},
		},
	})
}
