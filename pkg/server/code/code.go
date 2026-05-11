package code

import (
	"html/template"
	"log/slog"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const (
	contextLimit = 10
	viewList     = "list"
	viewThread   = "thread"
)

type Deps struct {
	Comments   db.CommentStore
	CommitLog  func(r repo.Repo) ([]git.Commit, error)
	Diff       func(r repo.Repo) ([]git.File, error)
	Home       string
	HunkCommit func(r repo.Repo, path string, newStart, newCount int) (git.Commit, error)
	ListRepos  func() ([]repo.Repo, error)
	LookupRepo func(slug string) (repo.Repo, bool)
	ShowFile   func(r repo.Repo, path string) ([]string, error)
}

func DefaultDeps(home string) Deps {
	return Deps{
		CommitLog: func(r repo.Repo) ([]git.Commit, error) {
			return git.CommitLog(r.Path, baseRef(r), "HEAD")
		},
		Diff: func(r repo.Repo) ([]git.File, error) {
			return git.Diff(r.Path, baseRef(r), "HEAD")
		},
		Home: home,
		HunkCommit: func(r repo.Repo, path string, newStart, newCount int) (git.Commit, error) {
			return git.HunkCommit(r.Path, baseRef(r), "HEAD", path, newStart, newCount)
		},
		ListRepos:  func() ([]repo.Repo, error) { return repo.Scan(home) },
		LookupRepo: func(slug string) (repo.Repo, bool) { return repo.Find(home, slug) },
		ShowFile: func(r repo.Repo, path string) ([]string, error) {
			return git.ShowFile(r.Path, "HEAD", path)
		},
	}
}

func baseRef(r repo.Repo) string {
	if r.Base == "" {
		return "main"
	}
	return r.Base
}

type Server struct {
	deps Deps
	tmpl *template.Template
}

func NewServer(deps Deps) (*Server, error) {
	if deps.Comments == nil {
		deps.Comments = db.NopCommentStore{}
	}
	tmpl, err := ui.ParseCode(funcs)
	if err != nil {
		return nil, errors.Wrap(err, "parse templates")
	}
	return &Server{deps: deps, tmpl: tmpl}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /{org}/{name}", s.handleDiff)
	mux.HandleFunc("GET /{org}/{name}/commits", s.handleCommits)
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

type overviewPage struct {
	Error string
	Home  string
	Repos []repo.Repo
}

type diffPage struct {
	Comments int
	Error    string
	Files    []fileRow
	Repo     repo.Repo
}

type commitsPage struct {
	Comments int
	Commits  []git.Commit
	Error    string
	Repo     repo.Repo
}

type fileRow struct {
	Adds     int
	Binary   bool
	Comments int
	Dels     int
	Hunks    []*hunkBlock
	Path     string
	Slug     string
	Status   git.FileStatus
}

type hunkBlock struct {
	Commit   git.Commit
	Hunk     *git.Hunk
	Lang     string
	Lines    []lineRow
	PrevSpot *expandSpot
	Virtual  bool
}

type lineRow struct {
	Anchor     string
	AnchorLine int
	Comments   []db.Comment
	Lang       string
	Line       git.Line
	Path       string
	Side       string
	Slug       string
}

type commentThread struct {
	Anchor   string
	Comments []db.Comment
	Slug     string
}

type newCommentForm struct {
	Anchor string
	Line   int
	Path   string
	Side   string
	Slug   string
}

type commentItem struct {
	Anchor  string
	Comment db.Comment
	Slug    string
	View    string
}

type editCommentForm struct {
	Anchor  string
	Comment db.Comment
	Slug    string
	View    string
}

type commentListPage struct {
	Comments int
	Error    string
	Files    []commentListFile
	Repo     repo.Repo
}

type commentListFile struct {
	Comments []db.Comment
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

type contextResponse struct {
	Continuation *expandSpot
	Direction    string
	From         int
	HunkKey      string
	Lang         string
	Lines        []string
	Offset       int
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	vm := overviewPage{Home: s.deps.Home}
	repos, err := s.deps.ListRepos()
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Repos = repos
	}
	s.render(w, "overview", vm)
}

func (s *Server) handleCommits(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := commitsPage{Comments: s.countComments(slug, rp.Branch), Repo: rp}
	if s.deps.CommitLog == nil {
		vm.Error = "commits not available"
		s.render(w, "commits", vm)
		return
	}
	commits, err := s.deps.CommitLog(rp)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, "commits", vm)
		return
	}
	vm.Commits = commits
	s.render(w, "commits", vm)
}

func (s *Server) countComments(slug, branch string) int {
	if s.deps.Comments == nil {
		return 0
	}
	cs, err := s.deps.Comments.ListComments(slug, branch)
	if err != nil {
		return 0
	}
	return len(cs)
}

func (s *Server) handleDiff(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := diffPage{Repo: rp}
	files, err := s.deps.Diff(rp)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, "diff", vm)
		return
	}
	comments, err := s.deps.Comments.ListComments(slug, rp.Branch)
	if err != nil {
		slog.Warn("list comments failed", "slug", slug, "branch", rp.Branch, "error", err.Error())
	}
	vm.Comments = len(comments)
	byAnchor := groupCommentsByAnchor(comments)
	byPath := groupCommentsByPath(comments)
	for _, f := range files {
		row := s.buildFileRow(rp, f, slug, byAnchor)
		row.Comments = byPath[row.Path]
		vm.Files = append(vm.Files, row)
	}
	s.render(w, "diff", vm)
}

func groupCommentsByPath(cs []db.Comment) map[string]int {
	out := make(map[string]int, len(cs))
	for _, c := range cs {
		out[c.Path]++
	}
	return out
}

func (s *Server) buildFileRow(rp repo.Repo, f git.File, slug string, byAnchor map[string][]db.Comment) fileRow {
	vm := fileRow{
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
		hb.Lines = buildLineRows(slug, f.Path(), lang, h.Lines, byAnchor)
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
	s.render(w, "comment-form", newCommentForm{
		Anchor: db.CommentAnchor(path, side, line),
		Line:   line,
		Path:   path,
		Side:   side,
		Slug:   slug,
	})
}

func (s *Server) handleCommentCreate(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
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
	if _, err := s.deps.Comments.AddComment(slug, rp.Branch, path, side, line, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderThread(w, slug, rp.Branch, path, side, line)
}

func (s *Server) handleCommentDelete(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	if _, ok := s.deps.LookupRepo(slug); !ok {
		http.NotFound(w, r)
		return
	}
	id := r.PathValue("id")
	if err := s.deps.Comments.DeleteComment(slug, id); err != nil {
		if db.IsCommentNotFound(err) {
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
	c, err := s.deps.Comments.GetComment(slug, r.PathValue("id"))
	if err != nil {
		if db.IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, "comment-edit-form", editCommentForm{
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
	vm := commentListPage{Repo: rp}
	comments, err := s.deps.Comments.ListComments(slug, rp.Branch)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, "comment-list-page", vm)
		return
	}
	vm.Comments = len(comments)
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
	current, err := s.deps.Comments.GetComment(slug, id)
	if err != nil {
		if db.IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resolveBody := ""
	if !current.Resolved {
		resolveBody = strings.TrimSpace(r.FormValue("body"))
	}
	updated, err := s.deps.Comments.SetCommentResolved(slug, id, !current.Resolved, resolveBody)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	c, err := s.deps.Comments.GetComment(slug, r.PathValue("id"))
	if err != nil {
		if db.IsCommentNotFound(err) {
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
	updated, err := s.deps.Comments.UpdateCommentBody(slug, r.PathValue("id"), body)
	if err != nil {
		if db.IsCommentNotFound(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.renderCommentItem(w, updated, slug, viewParam(r.FormValue("view")))
}

func (s *Server) renderCommentItem(w http.ResponseWriter, c db.Comment, slug, view string) {
	vm := commentItem{Anchor: c.Anchor(), Comment: c, Slug: slug, View: view}
	if view == viewList {
		s.render(w, "comment-card-item", vm)
		return
	}
	s.render(w, "comment-item", vm)
}

func (s *Server) renderThread(w http.ResponseWriter, slug, branch, path, side string, line int) {
	all, err := s.deps.Comments.ListComments(slug, branch)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	anchor := db.CommentAnchor(path, side, line)
	var matches []db.Comment
	for _, c := range all {
		if c.Anchor() == anchor {
			matches = append(matches, c)
		}
	}
	s.render(w, "comment-thread", commentThread{Anchor: anchor, Comments: matches, Slug: slug})
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

	vm := contextResponse{
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

func buildLineRows(slug, path, lang string, lines []git.Line, byAnchor map[string][]db.Comment) []lineRow {
	out := make([]lineRow, 0, len(lines))
	for _, l := range lines {
		side, line := lineAnchorParts(l)
		vm := lineRow{Lang: lang, Line: l, Path: path, Side: side, Slug: slug}
		if line > 0 {
			vm.Anchor = db.CommentAnchor(path, side, line)
			vm.AnchorLine = line
			vm.Comments = byAnchor[vm.Anchor]
		}
		out = append(out, vm)
	}
	return out
}

func groupCommentsByAnchor(cs []db.Comment) map[string][]db.Comment {
	out := make(map[string][]db.Comment, len(cs))
	for _, c := range cs {
		a := c.Anchor()
		out[a] = append(out[a], c)
	}
	return out
}

func groupCommentsByFile(cs []db.Comment) []commentListFile {
	byPath := make(map[string][]db.Comment)
	for _, c := range cs {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]commentListFile, 0, len(paths))
	for _, p := range paths {
		items := byPath[p]
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Line != items[j].Line {
				return items[i].Line < items[j].Line
			}
			if items[i].Side != items[j].Side {
				return items[i].Side < items[j].Side
			}
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		out = append(out, commentListFile{Comments: items, Path: p})
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
		slog.Info("code request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
