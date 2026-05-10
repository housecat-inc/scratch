package remote

import (
	"net/http"
	"net/http/httptest"
	"net/url"
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

	sessions       []*claudecode.Session
	startSessionFn func(name, dir string) (*claudecode.Session, error)
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
		SessionQR: func(id string) ([]byte, error) {
			if f.sessionQRErr != nil {
				return nil, f.sessionQRErr
			}
			return f.sessionQR, nil
		},
		StartLogin: func() (Login, error) {
			f.loginCalls++
			if f.loginErr != nil {
				return nil, f.loginErr
			}
			return f.login, nil
		},
		StartSession: func(name, dir string) (*claudecode.Session, error) {
			f.startCalls++
			if f.startSessionFn != nil {
				return f.startSessionFn(name, dir)
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

func TestIndex(t *testing.T) {
	tests := []struct {
		name      string
		fake      fakeDeps
		mustHave  []string
		mustMiss  []string
	}{
		{
			name: "fresh state shows install active, others pending",
			fake: fakeDeps{},
			mustHave: []string{
				"claude-control",
				">Install<", ">Pay<", ">Configure<",
				`class="step active" id="card-install"`,
				`class="step pending" id="card-login"`,
				`class="step pending" id="card-configure"`,
				`hx-post="/install"`,
			},
			mustMiss: []string{`hx-post="/configure"`, `hx-post="/login"`},
		},
		{
			name: "installed checks step 1, activates step 2",
			fake: fakeDeps{installed: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step active" id="card-login"`,
				`class="step pending" id="card-configure"`,
				`hx-post="/login"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/configure"`},
		},
		{
			name: "authenticated checks step 2, activates step 3",
			fake: fakeDeps{installed: true, authenticated: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step done" id="card-login"`,
				`class="step active" id="card-configure"`,
				`hx-post="/configure"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/login"`},
		},
		{
			name: "all done shows every step checked, no buttons",
			fake: fakeDeps{installed: true, configured: true, authenticated: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step done" id="card-login"`,
				`class="step done" id="card-configure"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/login"`},
		},
		{
			name: "agents behind shows update prompt",
			fake: fakeDeps{
				installed: true, configured: true, authenticated: true,
				agents: agents.State{Installed: true, Behind: 3},
			},
			mustHave: []string{
				`class="step done" id="card-configure"`,
				"3 new agent changes available",
				`hx-post="/configure"`,
				">Update<",
			},
		},
		{
			name: "agents dirty shows local edits hint",
			fake: fakeDeps{
				installed: true, configured: true, authenticated: true,
				agents: agents.State{Installed: true, Behind: 2, Dirty: true},
			},
			mustHave: []string{"Local edits"},
			mustMiss: []string{`hx-post="/configure"`},
		},
		{
			name: "agents diverged shows local commits hint",
			fake: fakeDeps{
				installed: true, configured: true, authenticated: true,
				agents: agents.State{Installed: true, Behind: 1, Diverged: true},
			},
			mustHave: []string{"Local commits ahead"},
			mustMiss: []string{`hx-post="/configure"`},
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
				`class="step done" id="card-install"`,
				`class="step active" id="card-login" hx-swap-oob="true"`,
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
	a.Contains(body, `class="step done" id="card-configure"`)
	a.Contains(body, `id="card-sessions" hx-swap-oob="true"`)
	a.Contains(body, "Remote control sessions")
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
	a.Contains(rec.Body.String(), `class="step done" id="card-login"`)
	a.Contains(rec.Body.String(), `class="step active" id="card-configure" hx-swap-oob="true"`)
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

func TestSessionsCardHiddenUntilConfigured(t *testing.T) {
	tests := []struct {
		fake     fakeDeps
		mustHave []string
		mustMiss []string
		name     string
	}{
		{
			name:     "unconfigured hides session form",
			fake:     fakeDeps{installed: true, authenticated: true},
			mustMiss: []string{`hx-post="/sessions"`, "Remote control sessions"},
		},
		{
			name: "configured shows session form with default dir",
			fake: fakeDeps{installed: true, configured: true, authenticated: true},
			mustHave: []string{
				"Remote control sessions",
				`hx-post="/sessions"`,
				`value="/home/exedev"`,
				"No sessions running.",
			},
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

	fd := fakeDeps{installed: true, configured: true, authenticated: true}
	srv, err := NewServer(fd.deps())
	r.NoError(err)

	form := url.Values{"dir": {"/tmp/work"}, "name": {"alpha"}}
	req := httptest.NewRequest(http.MethodPost, "/sessions", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Equal(1, fd.startCalls)
	body := rec.Body.String()
	a.Contains(body, "https://claude.ai/code/session_abc")
	a.Contains(body, "alpha")
	a.Contains(body, "/tmp/work")
	a.Contains(body, `src="/sessions/id1/qr"`)
}

func TestSessionStartDefaultsDir(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	var gotDir string
	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		startSessionFn: func(name, dir string) (*claudecode.Session, error) {
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

	a.Equal("/home/exedev", gotDir)
}

func TestSessionStartError(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fd := fakeDeps{
		installed: true, configured: true, authenticated: true,
		startSessionFn: func(name, dir string) (*claudecode.Session, error) {
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
	a.Contains(rec.Body.String(), "No sessions running.")
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
