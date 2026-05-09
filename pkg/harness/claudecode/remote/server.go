package remote

import (
	"embed"
	"html/template"
	"net/http"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Deps struct {
	Authenticated func() (bool, error)
	Configure     func() error
	Configured    func() (bool, error)
	Install       func() error
	Installed     func() bool
	StartLogin    func() (Login, error)
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
	Authenticated  bool
	ConfigureError string
	Configured     bool
	InstallError   string
	Installed      bool
	LoginError     string
	LoginURL       string
}

func DefaultDeps() Deps {
	return Deps{
		Authenticated: claudecode.Authenticated,
		Configure:     claudecode.WriteDefaults,
		Configured:    claudecode.Configured,
		Install:       claudecode.EnsureInstalled,
		Installed:     claudecode.Installed,
		StartLogin: func() (Login, error) {
			return claudecode.StartLogin()
		},
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
	return mux
}

func (s *Server) handleConfigure(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if err := s.deps.Configure(); err != nil {
		vm.ConfigureError = err.Error()
		s.render(w, "configure-card", vm)
		return
	}
	vm.Configured = true
	s.render(w, "configure-card", vm)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	s.render(w, "index", s.viewModel())
}

func (s *Server) handleInstall(w http.ResponseWriter, r *http.Request) {
	vm := s.viewModel()
	if err := s.deps.Install(); err != nil {
		vm.InstallError = err.Error()
		s.render(w, "install-card", vm)
		return
	}
	vm.Installed = true
	s.render(w, "install-card", vm)
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
		vm.LoginError = err.Error()
		s.render(w, "login-card", vm)
		return
	}
	s.mu.Lock()
	s.login = l
	s.mu.Unlock()

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
		vm.LoginURL = l.URL()
		vm.LoginError = err.Error()
		s.render(w, "login-card", vm)
		return
	}

	_ = l.Close()
	s.mu.Lock()
	s.login = nil
	s.mu.Unlock()

	vm = s.viewModel()
	vm.Authenticated = true
	s.render(w, "login-card", vm)
}

func (s *Server) render(w http.ResponseWriter, name string, vm viewModel) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) viewModel() viewModel {
	vm := viewModel{Installed: s.deps.Installed()}
	if ok, err := s.deps.Configured(); err == nil {
		vm.Configured = ok
	}
	if ok, err := s.deps.Authenticated(); err == nil {
		vm.Authenticated = ok
	}

	s.mu.Lock()
	if s.login != nil {
		vm.LoginURL = s.login.URL()
	}
	s.mu.Unlock()
	return vm
}
