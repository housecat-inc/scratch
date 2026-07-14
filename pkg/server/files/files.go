package files

import (
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const maxFileSize = 2 * 1024 * 1024

type Deps struct {
	Root  string
	Shell func() ui.ToolShellProps
}

func DefaultDeps(root string) Deps {
	return Deps{Root: root}
}

type Server struct {
	deps Deps
}

func NewServer(deps Deps) (*Server, error) {
	return &Server{deps: deps}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /read", s.handleRead)
	mux.HandleFunc("GET /tree", s.handleTree)
	mux.HandleFunc("POST /save", s.handleSave)
	return logging(mux)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	base := s.base(r)
	entries, err := s.readDir(base, "")
	vm := ui.FilesProps{
		Dir:     base,
		Entries: entries,
		Parent:  parentDir(base),
		Root:    rootLabel(base),
		Shell:   s.shell(),
	}
	if err != nil {
		vm.Error = err.Error()
	}
	s.render(w, r, ui.FilesPage(vm))
}

func (s *Server) base(r *http.Request) string {
	dir := strings.TrimSpace(r.URL.Query().Get("dir"))
	if dir == "" {
		return s.deps.Root
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return s.deps.Root
	}
	return abs
}

func parentDir(dir string) string {
	dir = filepath.Clean(dir)
	parent := filepath.Dir(dir)
	if parent == dir {
		return ""
	}
	return parent
}

func (s *Server) shell() ui.ToolShellProps {
	if s.deps.Shell == nil {
		return ui.ToolShellProps{}
	}
	return s.deps.Shell()
}

func (s *Server) handleTree(w http.ResponseWriter, r *http.Request) {
	base := s.base(r)
	rel := strings.TrimSpace(r.URL.Query().Get("path"))
	entries, err := s.readDir(base, rel)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.render(w, r, ui.FileTreeList(ui.FileTreeProps{Dir: base, Entries: entries}))
}

func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	abs, err := safeJoin(s.base(r), r.URL.Query().Get("path"))
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
	_, _ = w.Write(data)
}

func (s *Server) handleSave(w http.ResponseWriter, r *http.Request) {
	abs, err := safeJoin(s.base(r), r.URL.Query().Get("path"))
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

func (s *Server) readDir(base, rel string) ([]ui.FileEntry, error) {
	abs, err := safeJoin(base, rel)
	if err != nil {
		return nil, err
	}
	items, err := os.ReadDir(abs)
	if err != nil {
		return nil, errors.Wrapf(err, "read dir %s", abs)
	}
	out := make([]ui.FileEntry, 0, len(items))
	for _, it := range items {
		name := it.Name()
		if skipName(name) {
			continue
		}
		child := name
		if rel != "" {
			child = strings.TrimSuffix(rel, "/") + "/" + name
		}
		out = append(out, ui.FileEntry{Dir: it.IsDir(), Name: name, Path: child})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dir != out[j].Dir {
			return out[i].Dir
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func skipName(name string) bool {
	switch name {
	case "node_modules":
		return true
	}
	if !strings.HasPrefix(name, ".") {
		return false
	}
	switch name {
	case ".env.example", ".gitignore":
		return false
	}
	return true
}

func rootLabel(root string) string {
	if home, err := os.UserHomeDir(); err == nil && home == root {
		return "~"
	}
	return root
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
		return "", errors.New("path escapes root")
	}
	return absAbs, nil
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
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
