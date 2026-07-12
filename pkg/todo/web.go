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

type WebServer struct {
	log   *slog.Logger
	tasks *Service
}

func NewWebServer(tasks *Service, log *slog.Logger) *WebServer {
	if log == nil {
		log = slog.Default()
	}
	return &WebServer{log: log, tasks: tasks}
}

func (s *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleList)
	mux.HandleFunc("GET /tasks/{id}", s.handleShow)
	mux.HandleFunc("POST /tasks", s.handleCreate)
	mux.HandleFunc("POST /tasks/{id}", s.handleUpdate)
	mux.HandleFunc("POST /tasks/{id}/delete", s.handleDelete)
	mux.HandleFunc("POST /tasks/{id}/done", s.handleDone)
	return logging.Middleware(s.log, mux)
}

func (s *WebServer) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.fail(w, err)
		return
	}
	title := strings.TrimSpace(r.FormValue("title"))
	if title == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	task, err := s.tasks.Create(title)
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/tasks/"+strconv.FormatInt(task.ID, 10), http.StatusSeeOther)
}

func (s *WebServer) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	if err := s.tasks.Delete(id); err != nil {
		s.notFoundOr(w, err)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *WebServer) handleDone(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	if _, err := s.tasks.Toggle(id); err != nil {
		s.notFoundOr(w, err)
		return
	}
	http.Redirect(w, r, "/tasks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *WebServer) handleList(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, 0)
}

func (s *WebServer) handleShow(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	s.renderPage(w, r, id)
}

func (s *WebServer) handleUpdate(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, err)
		return
	}
	if title := strings.TrimSpace(r.FormValue("title")); title != "" {
		if _, err := s.tasks.Edit(id, title); err != nil {
			s.notFoundOr(w, err)
			return
		}
	}
	if _, err := s.tasks.EditDescription(id, strings.TrimSpace(r.FormValue("description"))); err != nil {
		s.notFoundOr(w, err)
		return
	}
	http.Redirect(w, r, "/tasks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *WebServer) fail(w http.ResponseWriter, err error) {
	httperr.Log(s.log, "todo web error", err)
	httperr.Error(w, err, http.StatusInternalServerError)
}

func (s *WebServer) notFoundOr(w http.ResponseWriter, err error) {
	if db.IsTaskNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.fail(w, err)
}

func (s *WebServer) props(selectedID int64) (ui.TodoProps, error) {
	view, err := s.tasks.ViewTask(FilterAll, selectedID)
	if err != nil {
		return ui.TodoProps{}, err
	}
	props := ui.TodoProps{
		ActiveCount: view.ActiveCount,
		Tasks:       view.Tasks,
	}
	if view.Detail != nil {
		props.Detail = &ui.TodoTaskDetail{Task: view.Detail.Task}
	}
	return props, nil
}

func (s *WebServer) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
	}
}

func (s *WebServer) renderPage(w http.ResponseWriter, r *http.Request, selectedID int64) {
	props, err := s.props(selectedID)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.TodoPage(props))
}

func taskPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
