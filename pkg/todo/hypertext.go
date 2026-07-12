package todo

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/db"
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
	mux.HandleFunc("GET /tasks/{id}/edit", s.handleEdit)
	mux.HandleFunc("PATCH /tasks/{id}", s.handleUpdate)
	mux.HandleFunc("DELETE /tasks/{id}", s.handleDelete)
	return s.logging(mux)
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
	s.renderMain(w, r, FilterAll, 0)
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("todo error", "error", err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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

func taskID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid task id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func toProps(v View, editingID int64) ui.TodoProps {
	return ui.TodoProps{
		ActiveCount:    v.ActiveCount,
		CompletedCount: v.CompletedCount,
		EditingID:      editingID,
		Filter:         string(v.Filter),
		Tasks:          v.Tasks,
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

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
		)
	})
}
