package code

import (
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
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
	ShowBlob   func(r repo.Repo, path string) ([]byte, error)
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
		ShowBlob: func(r repo.Repo, path string) ([]byte, error) {
			return git.ShowBlob(r.Path, "HEAD", path)
		},
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
}

func NewServer(deps Deps) (*Server, error) {
	if deps.Comments == nil {
		deps.Comments = db.NopCommentStore{}
	}
	return &Server{deps: deps}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleOverview)
	mux.HandleFunc("GET /{org}/{name}", s.handleDiff)
	mux.HandleFunc("GET /{org}/{name}/commits", s.handleCommits)
	mux.HandleFunc("GET /{org}/{name}/file/blob", s.handleBlob)
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


func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	vm := ui.OverviewProps{Home: s.deps.Home}
	repos, err := s.deps.ListRepos()
	if err != nil {
		vm.Error = err.Error()
	} else {
		vm.Repos = repos
	}
	s.render(w, r, ui.OverviewPage(vm))
}

func (s *Server) handleCommits(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := ui.CommitsProps{Comments: s.countComments(slug, rp.Branch), Repo: rp}
	if s.deps.CommitLog == nil {
		vm.Error = "commits not available"
		s.render(w, r, ui.CommitsPage(vm))
		return
	}
	commits, err := s.deps.CommitLog(rp)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, r, ui.CommitsPage(vm))
		return
	}
	vm.Commits = commits
	s.render(w, r, ui.CommitsPage(vm))
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
	vm := ui.DiffProps{Repo: rp}
	files, err := s.deps.Diff(rp)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, r, ui.DiffPage(vm))
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
	s.render(w, r, ui.DiffPage(vm))
}

func groupCommentsByPath(cs []db.Comment) map[string]int {
	out := make(map[string]int, len(cs))
	for _, c := range cs {
		out[c.Path]++
	}
	return out
}

func (s *Server) buildFileRow(rp repo.Repo, f git.File, slug string, byAnchor map[string][]db.Comment) ui.FileProps {
	vm := ui.FileProps{
		Adds:   f.Adds(),
		Binary: f.Binary,
		Dels:   f.Dels(),
		Path:   f.Path(),
		Slug:   slug,
		Status: f.Status,
	}

	hunks := append([]git.Hunk(nil), f.Hunks...)
	lang := langFor(f.Path())
	makeBlock := func(h *git.Hunk) *ui.HunkProps {
		hb := &ui.HunkProps{Hunk: h, Lang: lang}
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
			hb.PrevSpot = &ui.ExpandSpot{
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
		vm.Hunks = append(vm.Hunks, &ui.HunkProps{
			Hunk:    virtual,
			Lang:    lang,
			Virtual: true,
			PrevSpot: &ui.ExpandSpot{
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
	s.render(w, r, ui.CommentForm(ui.CommentFormProps{
		Anchor: db.CommentAnchor(path, side, line),
		Line:   line,
		Path:   path,
		Side:   side,
		Slug:   slug,
	}))
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
	s.renderThread(w, r, slug, rp.Branch, path, side, line)
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
	s.render(w, r, ui.EditCommentForm(ui.EditCommentFormProps{
		Anchor:  c.Anchor(),
		Comment: c,
		Slug:    slug,
		View:    viewParam(r.URL.Query().Get("view")),
	}))
}

func (s *Server) handleCommentList(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	vm := ui.CommentListProps{Repo: rp}
	comments, err := s.deps.Comments.ListComments(slug, rp.Branch)
	if err != nil {
		vm.Error = err.Error()
		s.render(w, r, ui.CommentListPage(vm))
		return
	}
	vm.Comments = len(comments)
	vm.Files = groupCommentsByFile(comments, slug)
	s.render(w, r, ui.CommentListPage(vm))
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
	s.renderCommentItem(w, r, updated, slug, viewParam(r.FormValue("view")))
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
	s.renderCommentItem(w, r, c, slug, viewParam(r.URL.Query().Get("view")))
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
	s.renderCommentItem(w, r, updated, slug, viewParam(r.FormValue("view")))
}

func (s *Server) renderCommentItem(w http.ResponseWriter, r *http.Request, c db.Comment, slug, view string) {
	vm := ui.CommentItemProps{Anchor: c.Anchor(), Comment: c, Slug: slug, View: view}
	if view == viewList {
		s.render(w, r, ui.CommentCardItem(vm))
		return
	}
	s.render(w, r, ui.CommentItem(vm))
}

func (s *Server) renderThread(w http.ResponseWriter, r *http.Request, slug, branch, path, side string, line int) {
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
	s.render(w, r, ui.CommentThread(ui.CommentThreadProps{Anchor: anchor, Comments: matches, Slug: slug}))
}

func (s *Server) handleBlob(w http.ResponseWriter, r *http.Request) {
	slug := r.PathValue("org") + "/" + r.PathValue("name")
	rp, ok := s.deps.LookupRepo(slug)
	if !ok {
		http.NotFound(w, r)
		return
	}
	path := r.URL.Query().Get("path")
	if path == "" || strings.Contains(path, "..") {
		http.Error(w, "invalid path", http.StatusBadRequest)
		return
	}
	if s.deps.ShowBlob == nil {
		http.Error(w, "blob not available", http.StatusNotFound)
		return
	}
	data, err := s.deps.ShowBlob(rp, path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ct := mime.TypeByExtension(filepath.Ext(path))
	if ct == "" {
		ct = http.DetectContentType(data)
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Cache-Control", "no-store")
	w.Write(data)
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

	vm := ui.ContextProps{
		Direction: direction,
		From:      from,
		HunkKey:   hunkKey,
		Lang:      langFor(path),
		Offset:    offset,
	}
	if from <= to {
		vm.Lines = lines[from-1 : to]
	}

	cont := &ui.ExpandSpot{
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

	s.render(w, r, ui.ContextAttached(vm))
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

func buildLineRows(slug, path, lang string, lines []git.Line, byAnchor map[string][]db.Comment) []ui.DiffLineProps {
	out := make([]ui.DiffLineProps, 0, len(lines))
	for _, l := range lines {
		side, line := lineAnchorParts(l)
		vm := ui.DiffLineProps{Lang: lang, Line: l, Path: path, Side: side, Slug: slug}
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

func groupCommentsByFile(cs []db.Comment, slug string) []ui.CommentListFileProps {
	byPath := make(map[string][]db.Comment)
	for _, c := range cs {
		byPath[c.Path] = append(byPath[c.Path], c)
	}
	paths := make([]string, 0, len(byPath))
	for p := range byPath {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]ui.CommentListFileProps, 0, len(paths))
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
		out = append(out, ui.CommentListFileProps{Comments: items, Path: p, Slug: slug})
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
