package sessions

import (
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	codexBrowserLogin = `[hx-post="/codex/login"]:not([hx-vals])`
	codexCallbackForm = `form[hx-post="/codex/login/callback"]`
	codexCallbackURL  = "http://localhost:1455/auth/callback?code=c&state=s"
	codexDeviceLogin  = `[hx-post="/codex/login"][hx-vals]`
	codexPoll         = `[hx-get="/codex/login"]`
)

func TestSessionsFlowsHTML(t *testing.T) {
	t.Run("setup", func(t *testing.T) {
		t.Run("install cascades to the sign in step", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{})
			h.Load("/setup")
			h.Click(`[hx-post="/install"]`)
			h.ElementPresent("#card-install")
			h.ElementTextContains("body", "Update")
			h.ElementPresent(`[hx-post="/login"]`)
		})
		t.Run("install error stays on the install step", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{installErr: errors.New("boom")})
			h.Load("/setup")
			h.Click(`[hx-post="/install"]`)
			h.ElementTextContains("#card-install", "boom")
			h.ElementAbsent(`[hx-post="/login"]`)
		})
		t.Run("sign in submits the pasted code and cascades to configure", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{installed: true, login: &fakeLogin{url: "https://claude.com/auth?code=xyz"}})
			h.Load("/setup")
			h.Click(`[hx-post="/login"]`)
			h.Fill(`[name="code"]`, "abc")
			h.Submit(`form[hx-post="/login/code"]`)
			a.Equal(1, h.fd.loginCalls, "login started once")
			a.Equal("abc", h.fd.login.submitted, "code submitted")
			a.True(h.fd.login.closed, "login closed")
			h.ElementPresent("#card-login")
			h.ElementPresent(`[hx-post="/configure"]`)
		})
		t.Run("sign in keeps the retry form after an invalid code", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{installed: true, login: &fakeLogin{submitErr: errors.New("invalid code"), url: "https://claude.com/auth"}})
			h.Load("/setup")
			h.Click(`[hx-post="/login"]`)
			h.Fill(`[name="code"]`, "bad")
			h.Submit(`form[hx-post="/login/code"]`)
			h.ElementTextContains("#card-login", "invalid code")
			h.ElementPresent(`#card-login [href*="claude.com/auth"]`)
		})
		t.Run("codex browser sign in accepts a pasted callback", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{url: "https://auth.openai.com/oauth/authorize?state=s"}, installed: true})
			h.Load("/setup")
			h.Click(codexBrowserLogin)
			a.False(h.fd.codexDevice, "browser mode")
			h.ElementPresent(`#card-codex-login [href*="oauth/authorize"]`)
			h.ElementTextContains("#card-codex-login", "Sign in with ChatGPT")
			h.Click(codexPoll)
			h.ElementPresent(codexCallbackForm)
			h.Fill(`[name="callback"]`, codexCallbackURL)
			h.Submit(codexCallbackForm)
			a.Equal(codexCallbackURL, h.fd.codexLogin.callback, "callback forwarded")
			a.True(h.fd.codexLogin.closed, "codex login closed")
			h.ElementAbsent(codexCallbackForm)
			h.ElementPresent("#card-codex-login.is-done")
		})
		t.Run("codex browser sign in keeps the form after a bad paste", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{callbackErr: errors.New("paste the full URL"), url: "https://auth.openai.com/oauth/authorize"}, installed: true})
			h.Load("/setup")
			h.Click(codexBrowserLogin)
			h.Fill(`[name="callback"]`, "nope")
			h.Submit(codexCallbackForm)
			h.ElementTextContains("#card-codex-login", "paste the full URL")
			h.ElementPresent(codexCallbackForm)
		})
		t.Run("codex browser sign in shows codex errors and resets", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{url: "https://auth.openai.com/oauth/authorize"}, installed: true})
			h.Load("/setup")
			h.Click(codexBrowserLogin)
			h.fd.codexLogin.done = true
			h.fd.codexLogin.err = errors.New("token exchange failed")
			h.Click(codexPoll)
			h.ElementTextContains("#card-codex-login", "token exchange failed")
			h.ElementPresent(`[hx-post="/codex/login"]`)
		})
		t.Run("codex device sign in polls until done", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{code: "32LO-RHYUX", url: "https://auth.openai.com/codex/device"}, installed: true})
			h.Load("/setup")
			h.Click(codexDeviceLogin)
			a.True(h.fd.codexDevice, "device mode")
			h.ElementTextContains("#card-codex-login", "32LO-RHYUX")
			h.ElementAbsent(codexCallbackForm)
			h.Click(codexPoll)
			h.ElementTextContains("#card-codex-login", "32LO-RHYUX")
			h.fd.codexLogin.done = true
			h.Click(codexPoll)
			a.True(h.fd.codexLogin.closed, "codex login closed")
			h.ElementAbsent(codexPoll)
			h.ElementPresent("#card-codex-login.is-done")
		})
		t.Run("codex sign in cancel returns to connect", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{codexInstalled: true, codexLogin: &fakeCodexLogin{url: "https://auth.openai.com/oauth/authorize"}, installed: true})
			h.Load("/setup")
			h.Click(codexBrowserLogin)
			h.Click(`[hx-delete="/codex/login"]`)
			a.True(h.fd.codexLogin.closed, "codex login closed")
			h.ElementPresent(`[hx-post="/codex/login"]`)
			h.ElementAbsent(codexPoll)
		})
		t.Run("configure marks the step done", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{authenticated: true, installed: true})
			h.Load("/setup")
			h.Click(`[hx-post="/configure"]`)
			h.ElementPresent("#card-configure")
			h.ElementAbsent(`[hx-post="/configure"]`)
			a.Equal(1, h.fd.configureCalls, "configured once")
			a.True(h.fd.configured, "configured")
		})
	})

	t.Run("sessions", func(t *testing.T) {
		t.Run("start renders the running session", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, startedDeps())
			h.Load("/")
			h.Fill("#session-dir", "/tmp/work")
			h.Type(`[name="prompt"]`, "fix the login bug")
			h.Submit("#session-form")
			a.Equal(1, h.fd.startCalls, "started once")
			h.ElementTextContains("#card-sessions", "https://claude.ai/code/session_abc")
			h.ElementTextContains("#card-sessions", "fix-login-bug")
			h.ElementTextContains("#card-sessions", "/tmp/work")
			h.ElementPresent(`img[src="/sessions/id1/qr"]`)
		})
		t.Run("start defaults to the home directory", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc"}, nil
				},
			})
			h.Load("/")
			h.Submit("#session-form")
			a.Contains(h.Doc.Find("#card-sessions").Text(), mustHome(t))
		})
		t.Run("start surfaces errors", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{
				authenticated: true, configured: true, installed: true,
				startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
					return nil, errors.New("tmux missing")
				},
			})
			h.Load("/")
			h.Fill("#session-dir", "/tmp")
			h.Submit("#session-form")
			h.ElementTextContains("#card-sessions", "tmux missing")
		})
		t.Run("start with redirect sends the client home", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, startedDeps())
			h.Load("/")
			h.Doc.Find("#session-form").AppendHtml(`<input type="hidden" name="redirect" value="1"/>`)
			h.Submit("#session-form")
			a.Equal("/", h.Header.Get("HX-Redirect"))
		})
		t.Run("browse lists subdirectories", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return []string{"alpha", "beta"}, nil },
			})
			h.Load("/")
			h.Click(`[hx-get^="/sessions/picker"]`)
			h.ElementTextContains("#dir-picker", "alpha")
			h.ElementTextContains("#dir-picker", "beta")
			h.ElementTextContains("#dir-picker", "Up")
			h.ElementPresent(`#dir-picker [hx-get^="/sessions/picker?dir="]`)
		})
		t.Run("browse reports an empty directory", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{
				authenticated: true, configured: true, installed: true,
				listSubdirs: func(string) ([]string, error) { return nil, nil },
			})
			h.Load("/")
			h.Click(`[hx-get^="/sessions/picker"]`)
			h.ElementTextContains("#dir-picker", "No subdirectories")
		})
		t.Run("stop removes the session", func(t *testing.T) {
			a := assert.New(t)
			h := flowHTML(t, &fakeDeps{
				authenticated: true, configured: true, installed: true,
				sessions: []*claudecode.Session{{Dir: "/tmp", ID: "abcd", Name: "x", URL: "https://claude.ai/code/x"}},
			})
			h.Load("/")
			h.Click(`[hx-delete="/sessions/abcd"]`)
			a.Equal(1, h.fd.stopCalls, "stopped once")
			h.ElementTextContains("#card-sessions", "No sessions running yet.")
		})
		t.Run("points unfinished setup back to setup", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{installed: true})
			h.Load("/")
			h.ElementPresent(`a[href="/setup"]`)
			h.ElementTextContains("body", "Finish")
			h.ElementAbsent(`[hx-post="/sessions"]`)
		})
		t.Run("shows the start form once signed in", func(t *testing.T) {
			h := flowHTML(t, &fakeDeps{authenticated: true, installed: true})
			h.Load("/")
			h.ElementPresent(`[hx-post="/sessions"]`)
			h.ElementPresent(`[name="prompt"]`)
			h.ElementTextContains("body", "No sessions running yet.")
			h.ElementAbsent(`[name="name"]`)
		})
	})
}

type sessionHarness struct {
	*testkit.HTML
	fd *fakeDeps
}

func flowHTML(t *testing.T, fd *fakeDeps) *sessionHarness {
	t.Helper()
	s, err := NewServer(fd.deps())
	require.New(t).NoError(err)
	return &sessionHarness{HTML: testkit.NewHTML(t, s.Handler()), fd: fd}
}

func startedDeps() *fakeDeps {
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
