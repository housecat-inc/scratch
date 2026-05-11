package diffview

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
)

//go:embed templates/*.html
var templatesFS embed.FS

const contextLimit = 10

type Deps struct {
	Diff       func(r repo.Repo) ([]git.File, error)
	Home       string
	ListRepos  func() ([]repo.Repo, error)
	LookupRepo func(slug string) (repo.Repo, bool)
	ShowFile   func(r repo.Repo, path string) ([]string, error)
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
		ShowFile: func(r repo.Repo, path string) ([]string, error) {
			return git.ShowFile(r.Path, "HEAD", path)
		},
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
	mux.HandleFunc("GET /{org}/{name}/file/lines", s.handleContext)
	return logging(mux)
}

type overviewVM struct {
	Error string
	Home  string
	Repos []repo.Repo
}

type diffVM struct {
	Error string
	Files []fileVM
	Repo  repo.Repo
}

type fileVM struct {
	Adds   int
	Binary bool
	Dels   int
	Hunks  []*hunkBlock
	Path   string
	Slug   string
	Status git.FileStatus
}

type hunkBlock struct {
	Hunk     *git.Hunk
	PrevSpot *expandSpot
	Virtual  bool
}

type expandSpot struct {
	Bound     int
	Direction string
	From      int
	FullFrom  int
	FullTo    int
	HunkKey   string
	Offset    int
	Path      string
	Slug      string
	To        int
}

type contextVM struct {
	Continuation *expandSpot
	Direction    string
	From         int
	HunkKey      string
	Lines        []string
	Offset       int
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
		s.render(w, "diff", vm)
		return
	}
	for _, f := range files {
		vm.Files = append(vm.Files, s.buildFileVM(rp, f, slug))
	}
	s.render(w, "diff", vm)
}

func (s *Server) buildFileVM(rp repo.Repo, f git.File, slug string) fileVM {
	vm := fileVM{
		Adds:   f.Adds(),
		Binary: f.Binary,
		Dels:   f.Dels(),
		Path:   f.Path(),
		Slug:   slug,
		Status: f.Status,
	}

	hunks := append([]git.Hunk(nil), f.Hunks...)
	if f.Binary || f.Status == git.StatusAdded || f.Status == git.StatusDeleted || len(hunks) == 0 {
		for i := range hunks {
			vm.Hunks = append(vm.Hunks, &hunkBlock{Hunk: &hunks[i]})
		}
		return vm
	}

	lines, err := s.deps.ShowFile(rp, f.NewPath)
	if err != nil {
		slog.Warn("show file failed", "path", f.NewPath, "error", err.Error())
		for i := range hunks {
			vm.Hunks = append(vm.Hunks, &hunkBlock{Hunk: &hunks[i]})
		}
		return vm
	}
	fileEnd := len(lines)

	delta := 0
	for i := range hunks {
		h := &hunks[i]
		hb := &hunkBlock{Hunk: h}

		var prevBound int
		if i == 0 {
			prevBound = 1
		} else {
			prev := hunks[i-1]
			prevBound = prev.NewStart + prev.NewCount
		}
		prevEnd := h.NewStart - 1
		if prevEnd >= prevBound {
			hb.PrevSpot = &expandSpot{
				Bound:     prevBound,
				Direction: "up",
				From:      maxInt(prevBound, prevEnd-contextLimit+1),
				FullFrom:  prevBound,
				FullTo:    prevEnd,
				HunkKey:   h.Key(),
				Offset:    -delta,
				Path:      f.NewPath,
				Slug:      slug,
				To:        prevEnd,
			}
		}

		vm.Hunks = append(vm.Hunks, hb)
		delta += h.NewCount - h.OldCount
	}

	last := hunks[len(hunks)-1]
	lastEnd := last.NewStart + last.NewCount - 1
	if lastEnd < fileEnd {
		virtual := &git.Hunk{Header: "end:" + f.NewPath}
		vm.Hunks = append(vm.Hunks, &hunkBlock{
			Hunk:    virtual,
			Virtual: true,
			PrevSpot: &expandSpot{
				Bound:     lastEnd + 1,
				Direction: "up",
				From:      maxInt(lastEnd+1, fileEnd-contextLimit+1),
				FullFrom:  lastEnd + 1,
				FullTo:    fileEnd,
				HunkKey:   virtual.Key(),
				Offset:    -delta,
				Path:      f.NewPath,
				Slug:      slug,
				To:        fileEnd,
			},
		})
	}

	return vm
}

func (s *Server) handleContext(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	path := q.Get("path")
	from := atoi(q.Get("from"))
	to := atoi(q.Get("to"))
	direction := q.Get("direction")
	bound := atoi(q.Get("bound"))
	offset := atoi(q.Get("offset"))
	hunkKey := q.Get("hunk")

	lines, err := s.deps.ShowFile(rp, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if from < 1 {
		from = 1
	}
	if to > len(lines) {
		to = len(lines)
	}

	vm := contextVM{
		Direction: direction,
		From:      from,
		HunkKey:   hunkKey,
		Offset:    offset,
	}
	if from <= to {
		vm.Lines = lines[from-1 : to]
	}

	switch direction {
	case "up":
		if from > bound {
			newTo := from - 1
			newFrom := maxInt(bound, from-contextLimit)
			vm.Continuation = &expandSpot{
				Bound:     bound,
				Direction: "up",
				From:      newFrom,
				FullFrom:  bound,
				FullTo:    newTo,
				HunkKey:   hunkKey,
				Offset:    offset,
				Path:      path,
				Slug:      slug,
				To:        newTo,
			}
		}
	case "down":
		if to < bound {
			newFrom := to + 1
			newTo := minInt(bound, to+contextLimit)
			vm.Continuation = &expandSpot{
				Bound:     bound,
				Direction: "down",
				From:      newFrom,
				FullFrom:  newFrom,
				FullTo:    bound,
				HunkKey:   hunkKey,
				Offset:    offset,
				Path:      path,
				Slug:      slug,
				To:        newTo,
			}
		}
	}

	s.render(w, "context-fragment-attached", vm)
}

func (s *Server) render(w http.ResponseWriter, name string, vm any) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := s.tmpl.ExecuteTemplate(w, name, vm); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
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
	"lineCount": func(from, to int) int { return to - from + 1 },
	"lineMarker": func(k git.LineKind) string {
		switch k {
		case git.LineAdd:
			return "+"
		case git.LineDelete:
			return "-"
		}
		return " "
	},
	"oldLineNum": func(from, offset, i int) int { return from + i + offset },
	"statusLabel": func(s git.FileStatus) string {
		switch s {
		case git.StatusAdded:
			return "new"
		case git.StatusDeleted:
			return "deleted"
		case git.StatusRenamed:
			return "renamed"
		default:
			return "modified"
		}
	},
	"statusColor": func(s git.FileStatus) string {
		switch s {
		case git.StatusAdded:
			return "text-green-700"
		case git.StatusDeleted:
			return "text-red-700"
		case git.StatusRenamed:
			return "text-blue-700"
		default:
			return "text-slate-500"
		}
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
		slog.Info("diffview request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
