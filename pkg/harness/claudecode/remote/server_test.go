package remote

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeLogin struct {
	closed     bool
	submitErr  error
	submitted  string
	url        string
}

func (f *fakeLogin) Close() error { f.closed = true; return nil }
func (f *fakeLogin) SubmitCode(code string) error {
	f.submitted = code
	return f.submitErr
}
func (f *fakeLogin) URL() string { return f.url }

type fakeDeps struct {
	agents        agents.State
	authenticated bool
	configured    bool
	installed     bool

	configureErr error
	installErr   error
	loginErr     error
	login        *fakeLogin

	configureCalls int
	installCalls   int
	loginCalls     int

	listSubdirs    func(dir string) ([]string, error)
	sessions       []*claudecode.Session
	sessionTail    []string
	slugForPrompt  func(prompt string) string
	startSessionFn func(name, dir, prompt string) (*claudecode.Session, error)
	stopSessionFn  func(id string) error
	sessionQR      []byte
	sessionQRErr   error
	startCalls     int
	stopCalls      int
}

func (f *fakeDeps) deps() Deps {
	return Deps{
		AgentsStatus:  func() (agents.State, error) { return f.agents, nil },
		Authenticated: func() (bool, error) { return f.authenticated, nil },
		Configure: func() error {
			f.configureCalls++
			if f.configureErr != nil {
				return f.configureErr
			}
			f.configured = true
			return nil
		},
		Configured: func() (bool, error) { return f.configured, nil },
		Install: func() error {
			f.installCalls++
			if f.installErr != nil {
				return f.installErr
			}
			f.installed = true
			return nil
		},
		Installed: func() bool { return f.installed },
		ListSessions: func() []*claudecode.Session { return f.sessions },
		ListSubdirs: func(dir string) ([]string, error) {
			if f.listSubdirs != nil {
				return f.listSubdirs(dir)
			}
			return nil, nil
		},
		SessionQR: func(id string) ([]byte, error) {
			if f.sessionQRErr != nil {
				return nil, f.sessionQRErr
			}
			return f.sessionQR, nil
		},
		SessionTail: func(id string, lines int) []string { return f.sessionTail },
		SlugForPrompt: func(prompt string) string {
			if f.slugForPrompt != nil {
				return f.slugForPrompt(prompt)
			}
			return ""
		},
		StartLogin: func() (Login, error) {
			f.loginCalls++
			if f.loginErr != nil {
				return nil, f.loginErr
			}
			return f.login, nil
		},
		StartSession: func(name, dir, prompt string) (*claudecode.Session, error) {
			f.startCalls++
			if f.startSessionFn != nil {
				return f.startSessionFn(name, dir, prompt)
			}
			s := &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc", StartedAt: time.Now()}
			f.sessions = append(f.sessions, s)
			return s, nil
		},
		StopSession: func(id string) error {
			f.stopCalls++
			if f.stopSessionFn != nil {
				return f.stopSessionFn(id)
			}
			out := f.sessions[:0]
			for _, s := range f.sessions {
				if s.ID != id {
					out = append(out, s)
				}
			}
			f.sessions = out
			return nil
		},
	}
}

func TestSetupPage(t *testing.T) {
	tests := []struct {
		fake     fakeDeps
		mustHave []string
		mustMiss []string
		name     string
	}{
		{
			name: "fresh state shows install active, others pending",
			fake: fakeDeps{},
			mustHave: []string{
				"claude-control",
				">Install<", ">Pay<", ">Configure<",
				`id="card-install"`,
				`id="card-login"`,
				`id="card-configure"`,
				`hx-post="/install"`,
			},
			mustMiss: []string{`hx-post="/configure"`, `hx-post="/login"`},
		},
		{
			name: "installed checks step 1, activates step 2",
			fake: fakeDeps{installed: true},
			mustHave: []string{
				`hx-post="/login"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/configure"`},
		},
		{
			name: "authenticated checks step 2, activates step 3",
			fake: fakeDeps{installed: true, authenticated: true},
			mustHave: []string{
				`hx-post="/configure"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/login"`},
		},
		{
			name: "all done shows no buttons",
			fake: fakeDeps{installed: true, configured: true, authenticated: true},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/login"`, `hx-post="/configure"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			s, err := NewServer(tc.fake.deps())
			r.NoError(err)

			req := httptest.NewRequest(http.MethodGet, "/setup", nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			a.Equal(http.StatusOK, rec.Code)
			body := rec.Body.String()
			for _, want := range tc.mustHave {
				a.Contains(body, want)
			}
			for _, miss := range tc.mustMiss {
				a.NotContains(body, miss)
			}
		})
	}
}

func TestTopNav(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		active string
	}{
		{name: "root highlights sessions", path: "/", active: "sessions"},
		{name: "setup highlights setup", path: "/setup", active: "setup"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{}
			s, err := NewServer(fd.deps())
			r.NoError(err)
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			body := rec.Body.String()
			a.Contains(body, `>Sessions</a>`)
			a.Contains(body, `>Code</a>`)
			a.Contains(body, `>Setup</a>`)
			a.Contains(body, `href="/code/"`)

			activeNeedle := map[string]string{
				"sessions": `href="/" class="flex-1 text-center py-3 text-sm font-medium text-slate-900`,
				"setup":    `href="/setup" class="flex-1 text-center py-3 text-sm font-medium text-slate-900`,
			}[tc.active]
			a.Contains(body, activeNeedle)
		})
	}
}

func TestSessionsPageOmitsAgentsHints(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		agents: agents.State{Installed: true, Behind: 2, Dirty: true},
	}
	s, err := NewServer(fd.deps())
	r.NoError(err)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, miss := range []string{"Local changes in ./scratch", "Upstream changes to ./scratch", ">Commit & Push<", ">Pull<"} {
		a.NotContains(body, miss)
	}
}

func TestSessionStartRedirectsWhenRequested(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{installed: true, configured: true, authenticated: true}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	form := url.Values{"dir": {"/tmp"}, "prompt": {"do a thing"}, "redirect": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal("/", rec.Header().Get("HX-Redirect"))
	a.Empty(rec.Body.String(), "no body when redirecting")
}

func TestInstall(t *testing.T) {
	tests := []struct {
		err      error
		mustHave []string
		mustMiss []string
		name     string
	}{
		{
			name: "success checks install step and OOB-activates login step",
			mustHave: []string{
				`id="card-install"`,
				`id="card-login" hx-swap-oob="true"`,
				`hx-post="/login"`,
			},
			mustMiss: []string{`hx-post="/install"`},
		},
		{
			name:     "error renders error message",
			err:      errors.New("boom"),
			mustHave: []string{"boom"},
			mustMiss: []string{`hx-swap-oob`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{installErr: tc.err}
			s, err := NewServer(fd.deps())
			r.NoError(err)

			req := httptest.NewRequest(http.MethodPost, "/install", nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			a.Equal(http.StatusOK, rec.Code)
			body := rec.Body.String()
			for _, want := range tc.mustHave {
				a.Contains(body, want)
			}
			for _, miss := range tc.mustMiss {
				a.NotContains(body, miss)
			}
		})
	}
}

func TestConfigure(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{installed: true, authenticated: true}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/configure", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	a.Contains(body, `id="card-configure"`)
	a.NotContains(body, `hx-post="/configure"`, "configure step is done — button gone")
	a.Equal(1, fd.configureCalls)
	a.True(fd.configured)
}

func TestLoginFlow(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fl := &fakeLogin{url: "https://claude.com/auth?code=xyz"}
	fd := fakeDeps{installed: true, login: fl}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Contains(rec.Body.String(), fl.url)
	a.Contains(rec.Body.String(), `name="code"`)
	a.Equal(1, fd.loginCalls)

	req = httptest.NewRequest(http.MethodPost, "/login", nil)
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	a.Equal(1, fd.loginCalls, "second start should reuse existing login")

	form := url.Values{"code": {"abc"}}
	req = httptest.NewRequest(http.MethodPost, "/login/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal("abc", fl.submitted)
	a.True(fl.closed)
	a.Contains(rec.Body.String(), `id="card-login"`)
	a.Contains(rec.Body.String(), `id="card-configure" hx-swap-oob="true"`)
}

func TestLoginCodeInvalid(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fl := &fakeLogin{url: "https://claude.com/auth", submitErr: errors.New("invalid code")}
	fd := fakeDeps{installed: true, login: fl}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	a.Equal(http.StatusOK, rec.Code)

	form := url.Values{"code": {"bad"}}
	req = httptest.NewRequest(http.MethodPost, "/login/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec = httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	a.Contains(body, "invalid code")
	a.Contains(body, fl.url, "URL stays so user can retry")
}

func TestLoginCodeMissing(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{installed: true}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/login/code", strings.NewReader("code=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Contains(rec.Body.String(), "no login in progress")
}

func TestSessionsPageGate(t *testing.T) {
	tests := []struct {
		fake     fakeDeps
		mustHave []string
		mustMiss []string
		name     string
	}{
		{
			name:     "unconfigured points back to setup",
			fake:     fakeDeps{installed: true, authenticated: true},
			mustHave: []string{`href="/setup"`, "Finish"},
			mustMiss: []string{`hx-post="/sessions"`},
		},
		{
			name: "configured shows session form with default dir",
			fake: fakeDeps{installed: true, configured: true, authenticated: true},
			mustHave: []string{
				`hx-post="/sessions"`,
				`value="` + mustHome(t) + `"`,
				"No sessions running yet.",
				`name="prompt"`,
				`hx-get="/sessions/picker?dir=` + mustHome(t) + `"`,
			},
			mustMiss: []string{`name="name"`},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			s, err := NewServer(tc.fake.deps())
			r.NoError(err)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)

			body := rec.Body.String()
			for _, want := range tc.mustHave {
				a.Contains(body, want)
			}
			for _, miss := range tc.mustMiss {
				a.NotContains(body, miss)
			}
		})
	}
}

func TestSessionStart(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	var gotName, gotPrompt string
	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		slugForPrompt: func(p string) string {
			if p == "" {
				return ""
			}
			return "fix-login-bug"
		},
		startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
			gotName, gotPrompt = name, prompt
			return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/session_abc"}, nil
		},
	}
	fd.startSessionFn = wrapAppend(&fd, fd.startSessionFn)
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	form := url.Values{"dir": {"/tmp/work"}, "prompt": {"fix the login bug"}}
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal(1, fd.startCalls)
	a.Equal("fix-login-bug", gotName, "slug from prompt is passed as session name")
	a.Equal("fix the login bug", gotPrompt)
	body := rec.Body.String()
	a.Contains(body, "https://claude.ai/code/session_abc")
	a.Contains(body, "fix-login-bug")
	a.Contains(body, "/tmp/work")
	a.Contains(body, `src="/sessions/id1/qr"`)
}

func TestSessionStartNoPromptUsesEmptyName(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	var gotName string
	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		slugForPrompt: func(p string) string { return "should-not-be-called" },
		startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
			gotName = name
			return &claudecode.Session{ID: "x", Name: name, Dir: dir, URL: "https://claude.ai/code/x"}, nil
		},
	}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader("dir=/tmp"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Empty(gotName, "no prompt — slug call is skipped, falls back to manager default")
}

func TestSessionStartDefaultsDir(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	var gotDir string
	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
			gotDir = dir
			return &claudecode.Session{ID: "id1", Name: name, Dir: dir, URL: "https://claude.ai/code/d"}, nil
		},
	}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(""))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(mustHome(t), gotDir)
}

func wrapAppend(fd *fakeDeps, inner func(name, dir, prompt string) (*claudecode.Session, error)) func(name, dir, prompt string) (*claudecode.Session, error) {
	return func(name, dir, prompt string) (*claudecode.Session, error) {
		s, err := inner(name, dir, prompt)
		if err == nil && s != nil {
			fd.sessions = append(fd.sessions, s)
		}
		return s, err
	}
}

func mustHome(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	require.New(t).NoError(err)
	return home
}

func TestSessionStartError(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		startSessionFn: func(name, dir, prompt string) (*claudecode.Session, error) {
			return nil, errors.New("tmux missing")
		},
	}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	form := url.Values{"dir": {"/tmp"}}
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Contains(rec.Body.String(), "tmux missing")
}

func TestSessionPicker(t *testing.T) {
	tests := []struct {
		name     string
		dir      string
		entries  []string
		err      error
		mustHave []string
		mustMiss []string
	}{
		{
			name:    "lists subdirs with up button when not at root",
			dir:     "/tmp",
			entries: []string{"alpha", "beta"},
			mustHave: []string{
				`hx-get="/sessions/picker?dir=/"`,
				"↑ Up",
				`hx-get="/sessions/picker?dir=/tmp/alpha"`,
				`hx-get="/sessions/picker?dir=/tmp/beta"`,
				`id="dir-text" hx-swap-oob="true"`,
				`id="session-dir"`,
				`value="/tmp"`,
			},
		},
		{
			name: "empty dir says no subdirectories",
			dir:  "/tmp",
			mustHave: []string{"No subdirectories"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{
				installed: true, configured: true, authenticated: true,
				listSubdirs: func(dir string) ([]string, error) {
					return tc.entries, tc.err
				},
			}
			srv, err := NewServer(fd.deps())
			r.NoError(err)

			req := httptest.NewRequest(http.MethodGet, "/sessions/picker?dir="+tc.dir, nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)

			a.Equal(http.StatusOK, rec.Code)
			body := rec.Body.String()
			for _, want := range tc.mustHave {
				a.Contains(body, want)
			}
			for _, miss := range tc.mustMiss {
				a.NotContains(body, miss)
			}
		})
	}
}

func TestSessionStop(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		sessions: []*claudecode.Session{{ID: "abcd", Name: "x", Dir: "/tmp", URL: "https://claude.ai/code/x"}},
	}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodDelete, "/sessions/abcd", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal(1, fd.stopCalls)
	a.Contains(rec.Body.String(), "No sessions running yet.")
}

func TestSessionQR(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{sessionQR: []byte{0x89, 'P', 'N', 'G', 0xff}}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/sessions/abcd/qr", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal("image/png", rec.Header().Get("Content-Type"))
	a.Equal([]byte{0x89, 'P', 'N', 'G', 0xff}, rec.Body.Bytes())
}

func TestSessionQRMissing(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{sessionQRErr: errors.New("session not found")}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodGet, "/sessions/nope/qr", nil)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusNotFound, rec.Code)
}

func TestLoginCascadesConfigureCard(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fl := &fakeLogin{url: "https://claude.com/auth"}
	fd := fakeDeps{installed: true, login: fl}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/login", nil)
	srv.Handler().ServeHTTP(httptest.NewRecorder(), req)

	form := url.Values{"code": {"abc"}}
	req = httptest.NewRequest(http.MethodPost, "/login/code", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	body := rec.Body.String()
	a.Contains(body, `id="card-configure"`)
	a.Contains(body, `hx-swap-oob="true"`)
	a.Contains(body, `hx-post="/configure"`)
}
