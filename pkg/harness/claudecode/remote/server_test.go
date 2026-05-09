package remote

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/cockroachdb/errors"
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
}

func (f *fakeDeps) deps() Deps {
	return Deps{
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
		StartLogin: func() (Login, error) {
			f.loginCalls++
			if f.loginErr != nil {
				return nil, f.loginErr
			}
			return f.login, nil
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
				"Install claude", "Unleash claude", "Connect subscription",
				`class="step active" id="card-install"`,
				`class="step pending" id="card-configure"`,
				`class="step pending" id="card-login"`,
				`hx-post="/install"`,
			},
			mustMiss: []string{`hx-post="/configure"`, `hx-post="/login"`},
		},
		{
			name: "installed checks step 1, activates step 2",
			fake: fakeDeps{installed: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step active" id="card-configure"`,
				`class="step pending" id="card-login"`,
				`hx-post="/configure"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/login"`},
		},
		{
			name: "unleashed checks step 2, activates step 3",
			fake: fakeDeps{installed: true, configured: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step done" id="card-configure"`,
				`class="step active" id="card-login"`,
				`hx-post="/login"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/configure"`},
		},
		{
			name: "all done shows every step checked, no buttons",
			fake: fakeDeps{installed: true, configured: true, authenticated: true},
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step done" id="card-configure"`,
				`class="step done" id="card-login"`,
			},
			mustMiss: []string{`hx-post="/install"`, `hx-post="/configure"`, `hx-post="/login"`},
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
			name: "success checks install step and OOB-activates configure step",
			mustHave: []string{
				`class="step done" id="card-install"`,
				`class="step active" id="card-configure" hx-swap-oob="true"`,
				`hx-post="/configure"`,
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

	fd := fakeDeps{installed: true}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/configure", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	a.Contains(body, `class="step done" id="card-configure"`)
	a.Contains(body, `class="step active" id="card-login" hx-swap-oob="true"`)
	a.Contains(body, `hx-post="/login"`)
	a.Equal(1, fd.configureCalls)
	a.True(fd.configured)
}

func TestLoginFlow(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fl := &fakeLogin{url: "https://claude.com/auth?code=xyz"}
	fd := fakeDeps{installed: true, configured: true, login: fl}
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
}

func TestLoginCodeInvalid(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	fl := &fakeLogin{url: "https://claude.com/auth", submitErr: errors.New("invalid code")}
	fd := fakeDeps{installed: true, configured: true, login: fl}
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

	fd := fakeDeps{installed: true, configured: true}
	s, err := NewServer(fd.deps())
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/login/code", strings.NewReader("code=abc"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Contains(rec.Body.String(), "no login in progress")
}
