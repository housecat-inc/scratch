package todo

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/server/logging"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/uikit"
)

type WebServer struct {
	chat  *chat.Service
	log   *slog.Logger
	tasks *Service
}

func NewWebServer(tasks *Service, log *slog.Logger) *WebServer {
	return NewWebServerWithChat(tasks, nil, log)
}

func NewWebServerWithChat(tasks *Service, chat *chat.Service, log *slog.Logger) *WebServer {
	if log == nil {
		log = slog.Default()
	}
	return &WebServer{chat: chat, log: log, tasks: tasks}
}

func (s *WebServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handleList)
	mux.HandleFunc("GET /chats", s.handleChats)
	mux.HandleFunc("GET /tasks", s.handleTasks)
	mux.HandleFunc("GET /tasks/{id}", s.handleShow)
	mux.HandleFunc("POST /tasks", s.handleCreate)
	mux.HandleFunc("POST /tasks/{id}/archive", s.handleArchive)
	mux.HandleFunc("POST /tasks/{id}", s.handleUpdate)
	mux.HandleFunc("POST /tasks/{id}/delete", s.handleDelete)
	mux.HandleFunc("POST /tasks/{id}/done", s.handleDone)
	return logging.Middleware(s.log, mux)
}

func (s *WebServer) handleChats(w http.ResponseWriter, r *http.Request) {
	props, err := s.props(FilterAll, 0)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	props.Tasks = nil
	props.Title = "Chats"
	props.View = "chats"
	s.render(w, r, ui.TodoPage(props))
}

func (s *WebServer) handleArchive(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	task, err := s.tasks.Get(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if _, err := s.tasks.SetArchived(id, !task.Archived); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
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
	s.redirectBack(w, r)
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
	s.redirectBack(w, r)
}

func (s *WebServer) handleList(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, FilterActive, 0)
}

func (s *WebServer) handleShow(w http.ResponseWriter, r *http.Request) {
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	s.renderPage(w, r, FilterAll, id)
}

func (s *WebServer) handleTasks(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, FilterAll, 0)
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

func (s *WebServer) props(filter Filter, selectedID int64) (ui.TodoProps, error) {
	all, err := s.tasks.All()
	if err != nil {
		return ui.TodoProps{}, err
	}
	activeCount := 0
	tasks := make([]db.Task, 0, len(all))
	for _, task := range all {
		if !task.Archived && !task.Completed {
			activeCount++
		}
		if filter == FilterActive && (task.Archived || task.Completed) {
			continue
		}
		tasks = append(tasks, task)
	}
	props := ui.TodoProps{
		ActiveCount: activeCount,
		Title:       todoTitle(filter),
		Tasks:       tasks,
		TaskCount:   len(all),
		View:        todoView(filter),
	}
	if s.chat != nil {
		chatItems, err := s.chatItems()
		if err != nil {
			return ui.TodoProps{}, err
		}
		props.ChatCount = len(chatItems)
		props.ChatItems = chatItems
		props.ChatOptions = chatOptions(s.chat.AgentNames())
	}
	if selectedID != 0 {
		task, err := s.tasks.Get(selectedID)
		if err != nil {
			return ui.TodoProps{}, err
		}
		props.Detail = &ui.TodoTaskDetail{Task: task}
	}
	return props, nil
}

func (s *WebServer) chatItems() ([]ui.TodoChatItem, error) {
	threads, err := s.chat.Threads()
	if err != nil {
		return nil, err
	}
	items := make([]ui.TodoChatItem, 0, len(threads))
	for _, thread := range threads {
		prompt, err := s.chat.ThreadPrompt(thread.ID)
		if err != nil {
			return nil, err
		}
		title := thread.Title
		if title == "" {
			title = "New chat"
		}
		items = append(items, ui.TodoChatItem{
			Description: chat.FriendlyDescription(prompt),
			ID:          thread.ID,
			Title:       title,
			When:        todoWhen(thread.UpdatedAt.Time),
		})
	}
	return items, nil
}

func chatOptions(agents []string) []uikit.SelectOption {
	options := chat.ProviderModelOptions(agents)
	out := make([]uikit.SelectOption, 0, len(options))
	for _, option := range options {
		out = append(out, uikit.SelectOption{Label: option.Label, Value: option.Value})
	}
	return out
}

func (s *WebServer) redirectBack(w http.ResponseWriter, r *http.Request) {
	back := r.FormValue("back")
	if back == "" {
		back = r.Header.Get("Referer")
	}
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *WebServer) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
	}
}

func (s *WebServer) renderPage(w http.ResponseWriter, r *http.Request, filter Filter, selectedID int64) {
	props, err := s.props(filter, selectedID)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.TodoPage(props))
}

func todoTitle(filter Filter) string {
	if filter == FilterActive {
		return "Inbox"
	}
	return "Tasks"
}

func todoWhen(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("Jan 2")
}

func todoView(filter Filter) string {
	if filter == FilterActive {
		return "inbox"
	}
	return "tasks"
}

func taskPathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}
