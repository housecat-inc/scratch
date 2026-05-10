package diffview

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
)

//go:embed templates/*.html
var templatesFS embed.FS

type Deps struct {
	Diff       func(r repo.Repo) ([]git.File, error)
	Home       string
	ListRepos  func() ([]repo.Repo, error)
	LookupRepo func(slug string) (repo.Repo, bool)
}

func DefaultDeps(home string) Deps {
	return Deps{
		Diff: func(r repo.Repo) ([]git.File, error) {
			base := r.Base
			if base == "" {
				base = "main"
			}
			return git.Diff(r.Path, base, "HEAD")
		},
		Home:       home,
		ListRepos:  func() ([]repo.Repo, error) { return repo.Scan(home) },
		LookupRepo: func(slug string) (repo.Repo, bool) { return repo.Find(home, slug) },
	}
}

type Server struct {
	deps Deps
	tmpl *template.Template
}

func NewServer(deps Deps) (*Server, error) {
	tmpl, err := template.New("").Funcs(funcs).ParseFS(templatesFS, "templates/*.html")
	if err != nil {
		return nil, errors.Wrap(err, "parse templates")
	}
	return &Server{deps: deps, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /{org}/{name}", s.handleDiff)
	return logging(mux)
}

type overviewVM struct {
	Error string
	Home  string
	Repos []repo.Repo
}

type diffVM struct {
	Error string
	Files []git.File
	Repo  repo.Repo
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	vm := overviewVM{Home: s.deps.Home}
	repos, err := s.deps.ListRepos()
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Repos = repos
	}
	s.render(w, "overview", vm)
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := diffVM{Repo: rp}
	files, err := s.deps.Diff(rp)
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Files = files
	}
	s.render(w, "diff", vm)
}

func (s *Server) render(w http.ResponseWriter, name string, vm any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var funcs = template.FuncMap{
	"addInt": func(a, b int) int { return a + b },
	"lineClass": func(k git.LineKind) string {
		switch k {
		case git.LineAdd:
			return "bg-green-50 text-green-900"
		case git.LineDelete:
			return "bg-red-50 text-red-900"
		}
		return ""
	},
	"lineMarker": func(k git.LineKind) string {
		switch k {
		case git.LineAdd:
			return "+"
		case git.LineDelete:
			return "-"
		}
		return " "
	},
	"prefix": strings.HasPrefix,
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
		slog.Info("diffview request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
