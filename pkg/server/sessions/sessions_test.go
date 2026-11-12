package sessions

import (
	"net/http"
	"net/http/httptest"
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
	closed    bool
	submitErr error
	submitted string
	url       string
}

func (f *fakeLogin) Close() error { f.closed = true; return nil }
func (f *fakeLogin) SubmitCode(code string) error {
	f.submitted = code
	return f.submitErr
}
func (f *fakeLogin) URL() string { return f.url }

type fakeCodexLogin struct {
	callback    string
	callbackErr error
	closed      bool
	code        string
	done        bool
	err         error
	url         string
}

func (f *fakeCodexLogin) Close() error { f.closed = true; return nil }
func (f *fakeCodexLogin) Code() string { return f.code }
func (f *fakeCodexLogin) Done() bool   { return f.done }
func (f *fakeCodexLogin) Err() error   { return f.err }
func (f *fakeCodexLogin) SubmitCallback(url string) error {
	f.callback = url
	if f.callbackErr != nil {
		return f.callbackErr
	}
	f.done = true
	return nil
}
func (f *fakeCodexLogin) URL() string { return f.url }

type fakeDeps struct {
	agents             agents.State
	authenticated      bool
	claudeVersion      string
	codexAuthenticated bool
	codexInstalled     bool
	codexVersion       string
	configured         bool
	installed          bool

	configureErr  error
	installErr    error
	loginErr      error
	login         *fakeLogin
	codexDevice   bool
	codexLogin    *fakeCodexLogin
	codexLoginErr error

	configureCalls int
	installCalls   int
	loginCalls     int

	listSubdirs    func(dir string) ([]string, error)
	sessions       []*claudecode.Session
	sessionLast    string
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
		AgentsStatus:       func() (agents.State, error) { return f.agents, nil },
		Authenticated:      func() (bool, error) { return f.authenticated, nil },
		ClaudeVersion:      func() string { return f.claudeVersion },
		CodexAuthenticated: func() (bool, error) { return f.codexAuthenticated, nil },
		CodexInstalled:     func() bool { return f.codexInstalled },
		CodexVersion:       func() string { return f.codexVersion },
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
		Installed:    func() bool { return f.installed },
		ListSessions: func() []*claudecode.Session { return f.sessions },
		ListSubdirs: func(dir string) ([]string, error) {
			if f.listSubdirs != nil {
				return f.listSubdirs(dir)
			}
			return nil, nil
		},
		SessionLastMessage: func(id string) string { return f.sessionLast },
		SessionQR: func(id string) ([]byte, error) {
			if f.sessionQRErr != nil {
				return nil, f.sessionQRErr
			}
			return f.sessionQR, nil
		},
		SlugForPrompt: func(prompt string) string {
			if f.slugForPrompt != nil {
				return f.slugForPrompt(prompt)
			}
			return ""
		},
		StartCodexLogin: func(device bool) (CodexLogin, error) {
			f.codexDevice = device
			if f.codexLoginErr != nil {
				return nil, f.codexLoginErr
			}
			return f.codexLogin, nil
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

func TestSessions(t *testing.T) {
	t.Run("nav", func(t *testing.T) {
		tests := []struct {
			active string
			name   string
			path   string
		}{
			{name: "lists chats instead of sessions on the root page", path: "/"},
			{active: `href="/setup" title="Setup" aria-current="page"`, name: "highlights setup on the setup page", path: "/setup"},
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
				a.Contains(body, `title="Chats"`)
				a.NotContains(body, `title="Sessions"`)
				a.Contains(body, `title="Code review"`)
				a.Contains(body, `title="Setup"`)
				a.Contains(body, `href="/code/"`)
				a.Contains(body, tc.active)
			})
		}
	})

	t.Run("sessions page", func(t *testing.T) {
		t.Run("omits agents repo hints", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{
				agents:        agents.State{Behind: 2, Dirty: true, Installed: true},
				authenticated: true, configured: true, installed: true,
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
		})
		t.Run("serves the sessions route when not mounted at root", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{authenticated: true, configured: true, installed: true}
			s, err := NewServer(fd.deps())
			r.NoError(err)

			mux := http.NewServeMux()
			s.Register(mux, false)

			req := httptest.NewRequest(http.MethodGet, "/sessions", nil)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)

			a.Equal(http.StatusOK, rec.Code)
			a.Contains(rec.Body.String(), "New session")
		})
	})

	t.Run("sign in", func(t *testing.T) {
		t.Run("reports no login in progress for a stray code", func(t *testing.T) {
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
		})
	})

	t.Run("start", func(t *testing.T) {
		t.Run("skips the prompt slug when no prompt is given", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			var gotName string
			fd := fakeDeps{
				authenticated: true, configured: true, installed: true,
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
		})
	})

	t.Run("QR code", func(t *testing.T) {
		t.Run("serves the session QR code as png", func(t *testing.T) {
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
		})
		t.Run("returns not found for an unknown session", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			fd := fakeDeps{sessionQRErr: errors.New("session not found")}
			srv, err := NewServer(fd.deps())
			r.NoError(err)

			req := httptest.NewRequest(http.MethodGet, "/sessions/nope/qr", nil)
			rec := httptest.NewRecorder()
			srv.Handler().ServeHTTP(rec, req)

			a.Equal(http.StatusNotFound, rec.Code)
		})
	})
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
