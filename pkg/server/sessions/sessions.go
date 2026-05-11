package sessions

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
	"github.com/housecat-inc/scratch/pkg/ui"
)

type Deps struct {
	AgentsStatus  func() (agents.State, error)
	Authenticated func() (bool, error)
	Configure     func() error
	Configured    func() (bool, error)
	Install       func() error
	Installed     func() bool
	ListSessions  func() []*claudecode.Session
	ListSubdirs   func(dir string) ([]string, error)
	SessionQR     func(id string) ([]byte, error)
	SessionTail   func(id string, lines int) []string
	SlugForPrompt func(prompt string) string
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
	home  string
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
	Nav            string
	Oob            bool
	SessionDir     string
	SessionError   string
	Sessions       []sessionView
}

type sessionView struct {
	Dir       string
	ID        string
	Name      string
	Prompt    string
	StartedAt time.Time
	Tail      []string
	URL       string
}

type pickerModel struct {
	Dir     string
	Entries []string
	Error   string
	Parent  string
	HasUp   bool
}

func DefaultDeps() Deps {
	mgr := claudecode.NewManager()
	if err := mgr.Recover(); err != nil {
		slog.Warn("session recovery failed", "error", err.Error())
	}
	return Deps{
		AgentsStatus:  agents.Status,
		Authenticated: claudecode.Authenticated,
		Configure:     claudecode.Configure,
		Configured:    claudecode.Configured,
		Install:       claudecode.EnsureInstalled,
		Installed:     claudecode.Installed,
		ListSessions:  mgr.List,
		ListSubdirs:   listSubdirs,
		SessionQR: func(id string) ([]byte, error) {
			s := mgr.Get(id)
			if s == nil {
				return nil, errors.New("session not found")
			}
			return s.QRPNG(256)
		},
		SessionTail: func(id string, lines int) []string { return mgr.Tail(id, lines) },
		SlugForPrompt: func(prompt string) string {
			return claudecode.SlugForPrompt(mgr.ClaudeBin, prompt)
		},
		StartLogin: func() (Login, error) {
			return claudecode.StartLogin()
		},
		StartSession: mgr.Start,
		StopSession:  mgr.Stop,
	}
}

func listSubdirs(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "read dir %s", dir)
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out, nil
}

func NewServer(deps Deps) (*Server, error) {
	tmpl, err := ui.ParseSessions()
	if err != nil {
		return nil, errors.Wrap(err, "parse templates")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, errors.Wrap(err, "user home dir")
	}
	return &Server{deps: deps, home: home, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleSessions)
	mux.HandleFunc("GET /setup", s.handleSetup)
	mux.HandleFunc("POST /configure", s.handleConfigure)
	mux.HandleFunc("POST /install", s.handleInstall)
	mux.HandleFunc("POST /login", s.handleLogin)
	mux.HandleFunc("POST /login/code", s.handleLoginCode)
	mux.HandleFunc("DELETE /sessions/{id}", s.handleSessionStop)
	mux.HandleFunc("GET /sessions/picker", s.handlePicker)
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
		slog.Info("sessions request",
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
	vm.Nav = "setup"
	if err := s.deps.Configure(); err != nil {
		slog.Error("configure failed", "error", err.Error())
		vm.ConfigureError = err.Error()
		s.render(w, "configure-card", vm)
		return
	}
	slog.Info("configured")
	vm = s.viewModel()
	vm.Nav = "setup"
	s.render(w, "configure-card", vm)
}

func (s *Server) handleSessions(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "sessions"
	s.render(w, "sessions-page", vm)
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "setup"
	s.render(w, "setup-page", vm)
}

func (s *Server) handlePicker(w http.ResponseWriter, r *http.Request) {
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		dir = s.home
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(s.home, dir)
	}
	dir = filepath.Clean(dir)

	pm := pickerModel{Dir: dir}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		pm.Error = "not a directory"
		s.render(w, "session-dir-picker", pm)
		return
	}
	if s.deps.ListSubdirs != nil {
		entries, err := s.deps.ListSubdirs(dir)
		if err != nil {
			pm.Error = err.Error()
		} else {
			pm.Entries = entries
		}
	}
	parent := filepath.Dir(dir)
	pm.Parent = parent
	pm.HasUp = parent != dir
	s.render(w, "session-dir-picker", pm)
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "setup"
	if err := s.deps.Install(); err != nil {
		slog.Error("install failed", "error", err.Error())
		vm.InstallError = err.Error()
		s.render(w, "install-card", vm)
		return
	}
	slog.Info("installed")
	vm = s.viewModel()
	vm.Nav = "setup"
	s.renderCascade(w, "install-card", vm, "login-card")
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "setup"

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
	vm.Nav = "setup"

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
	vm.Nav = "setup"
	vm.Authenticated = true
	s.renderCascade(w, "login-card", vm, "configure-card")
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "sessions"
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
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if dir == "" {
		dir = s.home
	}
	vm.SessionDir = dir

	name := ""
	if prompt != "" && s.deps.SlugForPrompt != nil {
		name = s.deps.SlugForPrompt(prompt)
	}

	sess, err := s.deps.StartSession(name, dir, prompt)
	if err != nil {
		slog.Error("start session failed", "error", err.Error())
		vm.SessionError = err.Error()
		s.render(w, "sessions-card", vm)
		return
	}
	slog.Info("session started", "id", sess.ID, "name", sess.Name, "url", sess.URL)
	if r.FormValue("redirect") != "" {
		w.Header().Set("HX-Redirect", "/")
		w.WriteHeader(http.StatusOK)
		return
	}
	vm = s.viewModel()
	vm.Nav = "sessions"
	vm.SessionDir = dir
	s.render(w, "sessions-card", vm)
}

func (s *Server) handleSessionStop(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	vm.Nav = "sessions"
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
	vm = s.viewModel()
	vm.Nav = "sessions"
	s.render(w, "sessions-card", vm)
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

func (s *Server) render(w http.ResponseWriter, name string, vm any) {
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
	vm := viewModel{Installed: s.deps.Installed(), SessionDir: s.home}
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
		sessions := s.deps.ListSessions()
		vm.Sessions = make([]sessionView, 0, len(sessions))
		for _, sess := range sessions {
			view := sessionView{
				Dir:       sess.Dir,
				ID:        sess.ID,
				Name:      sess.Name,
				Prompt:    sess.Prompt,
				StartedAt: sess.StartedAt,
				URL:       sess.URL,
			}
			if s.deps.SessionTail != nil {
				view.Tail = s.deps.SessionTail(sess.ID, 3)
			}
			vm.Sessions = append(vm.Sessions, view)
		}
	}
	return vm
}
