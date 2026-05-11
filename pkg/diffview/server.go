package diffview

import (
	"embed"
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
)

//go:embed templates/*.html
var templatesFS embed.FS

const (
	contextLimit = 10
	viewList     = "list"
	viewThread   = "thread"
)

type Deps struct {
	Comments   CommentStore
	Diff       func(r repo.Repo) ([]git.File, error)
	Home       string
	HunkCommit func(r repo.Repo, path string, newStart, newCount int) (git.Commit, error)
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
		Home: home,
		HunkCommit: func(r repo.Repo, path string, newStart, newCount int) (git.Commit, error) {
			base := r.Base
			if base == "" {
				base = "main"
			}
			return git.HunkCommit(r.Path, base, "HEAD", path, newStart, newCount)
		},
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
	if deps.Comments == nil {
		deps.Comments = nopCommentStore{}
	}
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
	mux.HandleFunc("GET /{org}/{name}/comments", s.handleCommentList)
	mux.HandleFunc("GET /{org}/{name}/comments/form", s.handleCommentForm)
	mux.HandleFunc("GET /{org}/{name}/comments/{id}", s.handleCommentShow)
	mux.HandleFunc("GET /{org}/{name}/comments/{id}/edit", s.handleCommentEdit)
	mux.HandleFunc("POST /{org}/{name}/comments", s.handleCommentCreate)
	mux.HandleFunc("POST /{org}/{name}/comments/{id}/resolve", s.handleCommentResolve)
	mux.HandleFunc("PUT /{org}/{name}/comments/{id}", s.handleCommentUpdate)
	mux.HandleFunc("DELETE /{org}/{name}/comments/{id}", s.handleCommentDelete)
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
	Commit   git.Commit
	Hunk     *git.Hunk
	Lang     string
	Lines    []lineVM
	PrevSpot *expandSpot
	Virtual  bool
}

type lineVM struct {
	Anchor     string
	AnchorLine int
	Comments   []Comment
	Lang       string
	Line       git.Line
	Path       string
	Side       string
	Slug       string
}

type threadVM struct {
	Anchor   string
	Comments []Comment
	Slug     string
}

type formVM struct {
	Anchor string
	Line   int
	Path   string
	Side   string
	Slug   string
}

type commentItemVM struct {
	Anchor  string
	Comment Comment
	Slug    string
	View    string
}

type commentEditVM struct {
	Anchor  string
	Comment Comment
	Slug    string
	View    string
}

type commentListVM struct {
	Error string
	Files []commentListFileVM
	Repo  repo.Repo
}

type commentListFileVM struct {
	Comments []Comment
	Path     string
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
	Lang         string
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
	comments, err := s.deps.Comments.List(slug)
	if err != nil {
		slog.Warn("list comments failed", "slug", slug, "error", err.Error())
	}
	byAnchor := groupCommentsByAnchor(comments)
	for _, f := range files {
		vm.Files = append(vm.Files, s.buildFileVM(rp, f, slug, byAnchor))
	}
	s.render(w, "diff", vm)
}

func (s *Server) buildFileVM(rp repo.Repo, f git.File, slug string, byAnchor map[string][]Comment) fileVM {
	vm := fileVM{
		Adds:   f.Adds(),
		Binary: f.Binary,
		Dels:   f.Dels(),
		Path:   f.Path(),
		Slug:   slug,
		Status: f.Status,
	}

	hunks := append([]git.Hunk(nil), f.Hunks...)
	lang := langFor(f.Path())
	makeBlock := func(h *git.Hunk) *hunkBlock {
		hb := &hunkBlock{Hunk: h, Lang: lang}
		hb.Lines = buildLineVMs(slug, f.Path(), lang, h.Lines, byAnchor)
		if s.deps.HunkCommit != nil && f.NewPath != "" {
			if c, err := s.deps.HunkCommit(rp, f.NewPath, h.NewStart, h.NewCount); err == nil {
				hb.Commit = c
			}
		}
		return hb
	}
	if f.Binary || f.Status == git.StatusAdded || f.Status == git.StatusDeleted || len(hunks) == 0 {
		for i := range hunks {
			vm.Hunks = append(vm.Hunks, makeBlock(&hunks[i]))
		}
		return vm
	}

	lines, err := s.deps.ShowFile(rp, f.NewPath)
	if err != nil {
		slog.Warn("show file failed", "path", f.NewPath, "error", err.Error())
		for i := range hunks {
			vm.Hunks = append(vm.Hunks, makeBlock(&hunks[i]))
		}
		return vm
	}
	fileEnd := len(lines)

	delta := 0
	for i := range hunks {
		h := &hunks[i]
		hb := makeBlock(h)

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
			Lang:    lang,
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

func (s *Server) handleCommentForm(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	path := q.Get("path")
	side := q.Get("side")
	line := atoi(q.Get("line"))
	if path == "" || !validSide(side) || line < 1 {
		http.Error(w, "invalid anchor", http.StatusBadRequest)
		return
	}
	s.render(w, "comment-form", formVM{
		Anchor: commentAnchor(path, side, line),
		Line:   line,
		Path:   path,
		Side:   side,
		Slug:   slug,
	})
}

func (s *Server) handleCommentCreate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	path := r.FormValue("path")
	side := r.FormValue("side")
	line := atoi(r.FormValue("line"))
	body := strings.TrimSpace(r.FormValue("body"))
	if path == "" || !validSide(side) || line < 1 {
		http.Error(w, "invalid anchor", http.StatusBadRequest)
		return
	}
	if body == "" {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	if _, err := s.deps.Comments.Add(slug, path, side, line, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderThread(w, slug, path, side, line)
}

func (s *Server) handleCommentDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	if err := s.deps.Comments.Delete(slug, id); err != nil {
		if IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleCommentEdit(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	c, err := s.deps.Comments.Get(slug, r.PathValue("id"))
	if err != nil {
		if IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "comment-edit-form", commentEditVM{
		Anchor:  c.Anchor(),
		Comment: c,
		Slug:    slug,
		View:    viewParam(r.URL.Query().Get("view")),
	})
}

func (s *Server) handleCommentList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := commentListVM{Repo: rp}
	comments, err := s.deps.Comments.List(slug)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, "comment-list-page", vm)
		return
	}
	vm.Files = groupCommentsByFile(comments)
	s.render(w, "comment-list-page", vm)
}

func (s *Server) handleCommentResolve(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	current, err := s.deps.Comments.Get(slug, id)
	if err != nil {
		if IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	updated, err := s.deps.Comments.SetResolved(slug, id, !current.Resolved)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.renderCommentItem(w, updated, slug, viewParam(r.FormValue("view")))
}

func (s *Server) handleCommentShow(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	c, err := s.deps.Comments.Get(slug, r.PathValue("id"))
	if err != nil {
		if IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderCommentItem(w, c, slug, viewParam(r.URL.Query().Get("view")))
}

func (s *Server) handleCommentUpdate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if body == "" {
		http.Error(w, "empty body", http.StatusBadRequest)
		return
	}
	updated, err := s.deps.Comments.Update(slug, r.PathValue("id"), body)
	if err != nil {
		if IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderCommentItem(w, updated, slug, viewParam(r.FormValue("view")))
}

func (s *Server) renderCommentItem(w http.ResponseWriter, c Comment, slug, view string) {
	vm := commentItemVM{Anchor: c.Anchor(), Comment: c, Slug: slug, View: view}
	if view == viewList {
		s.render(w, "comment-card-item", vm)
		return
	}
	s.render(w, "comment-item", vm)
}

func (s *Server) renderThread(w http.ResponseWriter, slug, path, side string, line int) {
	all, err := s.deps.Comments.List(slug)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	anchor := commentAnchor(path, side, line)
	var matches []Comment
	for _, c := range all {
		if c.Anchor() == anchor {
			matches = append(matches, c)
		}
	}
	s.render(w, "comment-thread", threadVM{Anchor: anchor, Comments: matches, Slug: slug})
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
		Lang:      langFor(path),
		Offset:    offset,
	}
	if from <= to {
		vm.Lines = lines[from-1 : to]
	}

	cont := &expandSpot{
		Bound:     bound,
		Direction: direction,
		HunkKey:   hunkKey,
		Offset:    offset,
		Path:      path,
		Slug:      slug,
	}
	switch direction {
	case "up":
		if from > bound {
			cont.From = maxInt(bound, from-contextLimit)
			cont.To = from - 1
			cont.FullFrom = bound
			cont.FullTo = from - 1
		} else {
			cont.From = bound
			cont.To = bound
			cont.FullFrom = bound
			cont.FullTo = bound
		}
	case "down":
		if to < bound {
			cont.From = to + 1
			cont.To = minInt(bound, to+contextLimit)
			cont.FullFrom = to + 1
			cont.FullTo = bound
		} else {
			cont.From = bound
			cont.To = bound
			cont.FullFrom = bound
			cont.FullTo = bound
		}
	default:
		cont.From = from
		cont.To = to
		cont.FullFrom = from
		cont.FullTo = to
	}
	vm.Continuation = cont

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

func buildLineVMs(slug, path, lang string, lines []git.Line, byAnchor map[string][]Comment) []lineVM {
	out := make([]lineVM, 0, len(lines))
	for _, l := range lines {
		side, line := lineAnchorParts(l)
		vm := lineVM{Lang: lang, Line: l, Path: path, Side: side, Slug: slug}
		if line > 0 {
			vm.Anchor = commentAnchor(path, side, line)
			vm.AnchorLine = line
			vm.Comments = byAnchor[vm.Anchor]
		}
		out = append(out, vm)
	}
	return out
}

func groupCommentsByAnchor(cs []Comment) map[string][]Comment {
	out := make(map[string][]Comment, len(cs))
	for _, c := range cs {
		a := c.Anchor()
		out[a] = append(out[a], c)
	}
	return out
}

func groupCommentsByFile(cs []Comment) []commentListFileVM {
	byPath := make(map[string][]Comment)
	for _, c := range cs {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]commentListFileVM, 0, len(paths))
	for _, p := range paths {
		items := byPath[p]
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Line != items[j].Line {
				return items[i].Line < items[j].Line
			}
			if items[i].Side != items[j].Side {
				return items[i].Side < items[j].Side
			}
			return items[i].Created.Before(items[j].Created)
		})
		out = append(out, commentListFileVM{Comments: items, Path: p})
	}
	return out
}

func lineAnchorParts(l git.Line) (side string, line int) {
	switch l.Kind {
	case git.LineDelete:
		return "old", l.OldLine
	default:
		return "new", l.NewLine
	}
}

func validSide(s string) bool { return s == "new" || s == "old" }

func viewParam(s string) string {
	if s == viewList {
		return viewList
	}
	return viewThread
}

func langFor(path string) string {
	switch filepath.Ext(path) {
	case ".bash", ".sh":
		return "language-bash"
	case ".css":
		return "language-css"
	case ".go":
		return "language-go"
	case ".html":
		return "language-html"
	case ".js":
		return "language-javascript"
	case ".json":
		return "language-json"
	case ".md":
		return "language-markdown"
	case ".py":
		return "language-python"
	case ".ts", ".tsx":
		return "language-typescript"
	case ".yaml", ".yml":
		return "language-yaml"
	}
	return ""
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
