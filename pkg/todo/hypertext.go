package todo

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/server/logging"
	"github.com/housecat-inc/scratch/pkg/ui"
)

type Server struct {
	log *slog.Logger
	svc *Service
}

func NewServer(svc *Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{log: log, svc: svc}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.HandleFunc("GET /{$}", s.handleIndex)
	mux.HandleFunc("POST /tasks", s.handleCreate)
	mux.HandleFunc("DELETE /tasks/completed", s.handleClearCompleted)
	mux.HandleFunc("GET /tasks/{id}", s.handleTask)
	mux.HandleFunc("GET /tasks/{id}/edit", s.handleEdit)
	mux.HandleFunc("POST /tasks/{id}/notes", s.handleCreateNote)
	mux.HandleFunc("POST /tasks/{id}/subitems", s.handleCreateSubitem)
	mux.HandleFunc("PATCH /tasks/{id}/subitems/{subitemID}", s.handleUpdateSubitem)
	mux.HandleFunc("DELETE /tasks/{id}/subitems/{subitemID}", s.handleDeleteSubitem)
	mux.HandleFunc("PATCH /tasks/{id}", s.handleUpdate)
	mux.HandleFunc("DELETE /tasks/{id}", s.handleDelete)
	return logging.Middleware(s.log, mux)
}

func (s *Server) handleClearCompleted(w http.ResponseWriter, r *http.Request) {
	if _, err := s.svc.ClearCompleted(); err != nil {
		s.fail(w, err)
		return
	}
	s.renderMain(w, r, FilterAll, 0)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title != "" {
		if _, err := s.svc.Create(title); err != nil {
			s.fail(w, err)
			return
		}
	}
	s.renderMain(w, r, FilterAll, 0)
}

func (s *Server) handleCreateNote(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	body := strings.TrimSpace(r.FormValue("body"))
	if body != "" {
		if _, err := s.svc.AddNote(id, body); err != nil {
			s.notFoundOr(w, err)
			return
		}
	}
	s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
}

func (s *Server) handleCreateSubitem(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title != "" {
		if _, err := s.svc.AddSubitem(id, title); err != nil {
			s.notFoundOr(w, err)
			return
		}
	}
	s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if err := s.svc.Delete(id); err != nil && !db.IsTaskNotFound(err) {
		s.fail(w, err)
		return
	}
	s.renderMain(w, r, FilterAll, 0)
}

func (s *Server) handleDeleteSubitem(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	subitemID, ok := pathInt64(w, r, "subitemID", "invalid subitem id")
	if !ok {
		return
	}
	if err := s.svc.DeleteSubitem(subitemID); err != nil && !db.IsTaskSubitemNotFound(err) {
		s.fail(w, err)
		return
	}
	s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
}

func (s *Server) handleEdit(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	s.renderMain(w, r, FilterAll, id)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.View(ParseFilter(r.URL.Query().Get("filter")))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, ui.TodoPage(toProps(view, 0)))
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
}

func (s *Server) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	if r.FormValue("title") != "" {
		if _, err := s.svc.Edit(id, strings.TrimSpace(r.FormValue("title"))); err != nil && !db.IsTaskNotFound(err) {
			s.fail(w, err)
			return
		}
	}
	if completed := r.FormValue("completed"); completed != "" {
		if _, err := s.svc.SetCompleted(id, completed == "true"); err != nil && !db.IsTaskNotFound(err) {
			s.fail(w, err)
			return
		}
	}
	if archived := r.FormValue("archived"); archived != "" {
		if _, err := s.svc.SetArchived(id, archived == "true"); err != nil && !db.IsTaskNotFound(err) {
			s.fail(w, err)
			return
		}
	}
	if starred := r.FormValue("starred"); starred != "" {
		if _, err := s.svc.SetStarred(id, starred == "true"); err != nil && !db.IsTaskNotFound(err) {
			s.fail(w, err)
			return
		}
	}
	if r.Header.Get("HX-Target") == "todo-shell" {
		s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
		return
	}
	s.renderMain(w, r, FilterAll, 0)
}

func (s *Server) handleUpdateSubitem(w http.ResponseWriter, r *http.Request) {
	id, ok := taskID(w, r)
	if !ok {
		return
	}
	subitemID, ok := pathInt64(w, r, "subitemID", "invalid subitem id")
	if !ok {
		return
	}
	if completed := r.FormValue("completed"); completed != "" {
		if _, err := s.svc.SetSubitemCompleted(subitemID, completed == "true"); err != nil && !db.IsTaskSubitemNotFound(err) {
			s.fail(w, err)
			return
		}
	}
	s.renderShell(w, r, ParseFilter(r.URL.Query().Get("filter")), id, 0)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	httperr.Log(s.log, "todo error", err)
	httperr.Error(w, err, http.StatusInternalServerError)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) renderMain(w http.ResponseWriter, r *http.Request, filter Filter, editingID int64) {
	view, err := s.svc.View(filter)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, ui.TodoMain(toProps(view, editingID)))
}

func (s *Server) renderShell(w http.ResponseWriter, r *http.Request, filter Filter, selectedID, editingID int64) {
	view, err := s.svc.ViewTask(filter, selectedID)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.TodoShell(toProps(view, editingID)))
}

func (s *Server) notFoundOr(w http.ResponseWriter, err error) {
	if db.IsTaskNotFound(err) || db.IsTaskSubitemNotFound(err) {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	s.fail(w, err)
}

func taskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	return pathInt64(w, r, "id", "invalid task id")
}

func pathInt64(w http.ResponseWriter, r *http.Request, name, msg string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil {
		http.Error(w, msg, http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func toProps(v View, editingID int64) ui.TodoProps {
	props := ui.TodoProps{
		ActiveCount:    v.ActiveCount,
		ArchivedCount:  v.ArchivedCount,
		CompletedCount: v.CompletedCount,
		EditingID:      editingID,
		Filter:         string(v.Filter),
		Tasks:          v.Tasks,
	}
	if v.Detail != nil {
		props.Detail = &ui.TodoDetailProps{
			Notes:    v.Detail.Notes,
			Subitems: v.Detail.Subitems,
			Task:     v.Detail.Task,
		}
	}
	return props
}
