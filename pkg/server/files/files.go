package files

import (
	"html/template"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const maxFileSize = 2 * 1024 * 1024

type Deps struct {
	Home       string
	ListRepos  func() ([]repo.Repo, error)
	LookupRepo func(slug string) (repo.Repo, bool)
}

func DefaultDeps(home string) Deps {
	return Deps{
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
	tmpl, err := ui.ParseFiles(funcs)
	if err != nil {
		return nil, errors.Wrap(err, "parse templates")
	}
	return &Server{deps: deps, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /{org}/{name}", s.handleEditor)
	mux.HandleFunc("GET /{org}/{name}/read", s.handleRead)
	mux.HandleFunc("GET /{org}/{name}/tree", s.handleTree)
	mux.HandleFunc("POST /{org}/{name}/save", s.handleSave)
	return logging(mux)
}

type overviewPage struct {
	Error string
	Home  string
	Repos []repo.Repo
}

type editorPage struct {
	Entries []entry
	Error   string
	Repo    repo.Repo
}

type treeFragment struct {
	Entries []entry
	Slug    string
}

type entry struct {
	Dir  bool
	Name string
	Path string
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	vm := overviewPage{Home: s.deps.Home}
	repos, err := s.deps.ListRepos()
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Repos = repos
	}
	s.render(w, "files-overview", vm)
}

func (s *Server) handleEditor(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	entries, err := readDir(rp.Path, "")
	vm := editorPage{Repo: rp, Entries: entries}
	if err != nil {
		vm.Error = err.Error()
	}
	s.render(w, "files-editor", vm)
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, err := readDir(rp.Path, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.render(w, "file-tree-list", treeFragment{Entries: entries, Slug: slug})
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	abs, err := safeJoin(rp.Path, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "is a directory", http.StatusBadRequest)
		return
	}
	if info.Size() > maxFileSize {
		http.Error(w, "file too large", http.StatusRequestEntityTooLarge)
		return
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("X-CM-Mode", cmMode(abs))
	_, _ = w.Write(data)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	abs, err := safeJoin(rp.Path, r.URL.Query().Get("path"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	info, err := os.Stat(abs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if info.IsDir() {
		http.Error(w, "is a directory", http.StatusBadRequest)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxFileSize))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := os.WriteFile(abs, body, info.Mode().Perm()); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func readDir(root, rel string) ([]entry, error) {
	abs, err := safeJoin(root, rel)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		return nil, errors.Wrapf(err, "read dir %s", abs)
	}
	out := make([]entry, 0, len(items))
	for _, it := range items {
		name := it.Name()
		if name == ".git" || strings.HasPrefix(name, ".") && name != ".gitignore" && name != ".env.example" {
			continue
		}
		child := name
		if rel != "" {
			child = rel + "/" + name
		}
		out = append(out, entry{Dir: it.IsDir(), Name: name, Path: child})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func safeJoin(root, rel string) (string, error) {
	clean := filepath.Clean("/" + strings.TrimPrefix(rel, "/"))
	abs := filepath.Join(root, clean)
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", errors.Wrap(err, "abs root")
	}
	absAbs, err := filepath.Abs(abs)
	if err != nil {
		return "", errors.Wrap(err, "abs path")
	}
	if absAbs != rootAbs && !strings.HasPrefix(absAbs, rootAbs+string(filepath.Separator)) {
		return "", errors.New("path escapes repo")
	}
	return absAbs, nil
}

func cmMode(path string) string {
	switch filepath.Ext(path) {
	case ".bash", ".sh":
		return "text/x-sh"
	case ".css":
		return "text/css"
	case ".go":
		return "text/x-go"
	case ".html", ".htm":
		return "text/html"
	case ".js", ".mjs":
		return "text/javascript"
	case ".json":
		return "application/json"
	case ".md", ".markdown":
		return "text/x-markdown"
	case ".py":
		return "text/x-python"
	case ".ts", ".tsx":
		return "text/typescript"
	case ".xml":
		return "application/xml"
	case ".yaml", ".yml":
		return "text/x-yaml"
	}
	return "text/plain"
}

func (s *Server) render(w http.ResponseWriter, name string, vm any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var funcs = template.FuncMap{
	"dict": func(values ...any) (map[string]any, error) {
		if len(values)%2 != 0 {
			return nil, errors.New("dict requires even number of args")
		}
		m := make(map[string]any, len(values)/2)
		for i := 0; i < len(values); i += 2 {
			key, ok := values[i].(string)
			if !ok {
				return nil, errors.Errorf("dict key %d is not a string", i)
			}
			m[key] = values[i+1]
		}
		return m, nil
	},
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
		slog.Info("files request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
