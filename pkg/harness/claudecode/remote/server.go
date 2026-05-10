package remote

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
)

//go:embed templates/*.html
var templatesFS embed.FS

const defaultSessionDir = "/home/exedev"

type Deps struct {
	AgentsStatus  func() (agents.State, error)
	Authenticated func() (bool, error)
	Configure     func() error
	Configured    func() (bool, error)
	Install       func() error
	Installed     func() bool
	ListSessions  func() []*claudecode.Session
	SessionQR     func(id string) ([]byte, error)
	StartLogin    func() (Login, error)
	StartSession  func(name, dir, prompt string) (*claudecode.Session, error)
	StopSession   func(id string) error
}

type Login interface {
	Close() error
	SubmitCode(code string) error
	URL() string
}

type Server struct {
	deps  Deps
	login Login
	mu    sync.Mutex
	tmpl  *template.Template
}

type viewModel struct {
	AgentsBehind   int
	AgentsDir      string
	AgentsDiverged bool
	AgentsDirty    bool
	Authenticated  bool
	ConfigureError string
	Configured     bool
	InstallError   string
	Installed      bool
	LoginError     string
	LoginURL       string
	Oob            bool
	SessionDir     string
	SessionError   string
	Sessions       []*claudecode.Session
}

func DefaultDeps() Deps {
	mgr := claudecode.NewManager()
	return Deps{
		AgentsStatus:  agents.Status,
		Authenticated: claudecode.Authenticated,
		Configure:     claudecode.Configure,
		Configured:    claudecode.Configured,
		Install:       claudecode.EnsureInstalled,
		Installed:     claudecode.Installed,
		ListSessions:  mgr.List,
		SessionQR: func(id string) ([]byte, error) {
			s := mgr.Get(id)
			if s == nil {
				return nil, errors.New("session not found")
			}
			return s.QRPNG(256)
		},
		StartLogin: func() (Login, error) {
			return claudecode.StartLogin()
		},
		StartSession: mgr.Start,
		StopSession:  mgr.Stop,
	}
}

func NewServer(deps Deps) (*Server, error) {
	tmpl, err := template.ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, errors.Wrap(err, "parse templates")
	}
	return &Server{deps: deps, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /configure", s.handleConfigure)
	mux.HandleFunc("POST /install", s.handleInstall)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /login/code", s.handleLoginCode)
	mux.HandleFunc("DELETE /sessions/{id}", s.handleSessionStop)
	mux.HandleFunc("GET /sessions/{id}/qr", s.handleSessionQR)
	mux.HandleFunc("POST /sessions", s.handleSessionStart)
	return logging(mux)
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if err := s.deps.Configure(); err != nil {
		slog.Error("configure failed", "error", err.Error())
		vm.ConfigureError = err.Error()
		s.render(w, "configure-card", vm)
		return
	}
	slog.Info("configured")
	vm = s.viewModel()
	s.renderCascade(w, "configure-card", vm, "sessions-card")
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, "index", s.viewModel())
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if err := s.deps.Install(); err != nil {
		slog.Error("install failed", "error", err.Error())
		vm.InstallError = err.Error()
		s.render(w, "install-card", vm)
		return
	}
	slog.Info("installed")
	vm = s.viewModel()
	s.renderCascade(w, "install-card", vm, "login-card")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()

	s.mu.Lock()
	existing := s.login
	s.mu.Unlock()
	if existing != nil {
		vm.LoginURL = existing.URL()
		s.render(w, "login-card", vm)
		return
	}

	l, err := s.deps.StartLogin()
	if err != nil {
		slog.Error("start login failed", "error", err.Error())
		vm.LoginError = err.Error()
		s.render(w, "login-card", vm)
		return
	}
	s.mu.Lock()
	s.login = l
	s.mu.Unlock()

	slog.Info("login started", "url", l.URL())
	vm.LoginURL = l.URL()
	s.render(w, "login-card", vm)
}

func (s *Server) handleLoginCode(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()

	if err := r.ParseForm(); err != nil {
		vm.LoginError = err.Error()
		s.render(w, "login-card", vm)
		return
	}
	code := strings.TrimSpace(r.FormValue("code"))
	if code == "" {
		vm.LoginError = "code is required"
		s.render(w, "login-card", vm)
		return
	}

	s.mu.Lock()
	l := s.login
	s.mu.Unlock()
	if l == nil {
		vm.LoginError = "no login in progress"
		s.render(w, "login-card", vm)
		return
	}

	if err := l.SubmitCode(code); err != nil {
		slog.Error("submit code failed", "error", err.Error())
		vm.LoginURL = l.URL()
		vm.LoginError = err.Error()
		s.render(w, "login-card", vm)
		return
	}

	_ = l.Close()
	s.mu.Lock()
	s.login = nil
	s.mu.Unlock()

	slog.Info("login succeeded")
	vm = s.viewModel()
	vm.Authenticated = true
	s.renderCascade(w, "login-card", vm, "configure-card")
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if s.deps.StartSession == nil {
		vm.SessionError = "sessions are not enabled"
		s.render(w, "sessions-card", vm)
		return
	}
	if err := r.ParseForm(); err != nil {
		vm.SessionError = err.Error()
		s.render(w, "sessions-card", vm)
		return
	}
	dir := strings.TrimSpace(r.FormValue("dir"))
	name := strings.TrimSpace(r.FormValue("name"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if dir == "" {
		dir = defaultSessionDir
	}
	vm.SessionDir = dir

	sess, err := s.deps.StartSession(name, dir, prompt)
	if err != nil {
		slog.Error("start session failed", "error", err.Error())
		vm.SessionError = err.Error()
		s.render(w, "sessions-card", vm)
		return
	}
	slog.Info("session started", "id", sess.ID, "url", sess.URL)
	vm = s.viewModel()
	vm.SessionDir = dir
	s.render(w, "sessions-card", vm)
}

func (s *Server) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if s.deps.StopSession == nil {
		vm.SessionError = "sessions are not enabled"
		s.render(w, "sessions-card", vm)
		return
	}
	id := r.PathValue("id")
	if err := s.deps.StopSession(id); err != nil {
		slog.Error("stop session failed", "id", id, "error", err.Error())
		vm.SessionError = err.Error()
		s.render(w, "sessions-card", vm)
		return
	}
	slog.Info("session stopped", "id", id)
	s.render(w, "sessions-card", s.viewModel())
}

func (s *Server) handleSessionQR(w http.ResponseWriter, r *http.Request) {
	if s.deps.SessionQR == nil {
		http.NotFound(w, r)
		return
	}
	png, err := s.deps.SessionQR(r.PathValue("id"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "public, max-age=300")
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

func (s *Server) render(w http.ResponseWriter, name string, vm viewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) renderCascade(w http.ResponseWriter, primary string, vm viewModel, oob ...string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, primary, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	vm.Oob = true
	for _, name := range oob {
		if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func (s *Server) viewModel() viewModel {
	vm := viewModel{Installed: s.deps.Installed(), SessionDir: defaultSessionDir}
	if dir, err := agents.Dir(); err == nil {
		vm.AgentsDir = dir
	}
	if ok, err := s.deps.Configured(); err == nil {
		vm.Configured = ok
	}
	if ok, err := s.deps.Authenticated(); err == nil {
		vm.Authenticated = ok
	}
	if vm.Configured && s.deps.AgentsStatus != nil {
		if state, err := s.deps.AgentsStatus(); err == nil {
			vm.AgentsBehind = state.Behind
			vm.AgentsDirty = state.Dirty
			vm.AgentsDiverged = state.Diverged
		}
	}

	s.mu.Lock()
	if s.login != nil {
		vm.LoginURL = s.login.URL()
	}
	s.mu.Unlock()

	if s.deps.ListSessions != nil {
		vm.Sessions = s.deps.ListSessions()
	}
	return vm
}
