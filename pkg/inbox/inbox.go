package inbox

import (
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/server/logging"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
)

type Server struct {
	chat  *chat.Service
	log   *slog.Logger
	tasks *todo.Service
}

func NewServer(tasks *todo.Service, chat *chat.Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{chat: chat, log: log, tasks: tasks}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.HandleFunc("GET /{$}", s.handleInbox)
	mux.HandleFunc("GET /chats", s.handleChats)
	mux.HandleFunc("GET /inbox", s.handleInbox)
	mux.HandleFunc("GET /starred", s.handleStarred)
	mux.HandleFunc("GET /tasks", s.handleTasks)
	mux.HandleFunc("GET /workflows", s.handleWorkflows)
	mux.HandleFunc("GET /inbox/chats/{id}", s.handleChat)
	mux.HandleFunc("GET /inbox/tasks/{id}", s.handleTask)
	mux.HandleFunc("GET /inbox/workflows/{id}", s.handleWorkflow)
	mux.HandleFunc("POST /compose", s.handleCompose)
	mux.HandleFunc("POST /inbox/chats/{id}/archive", s.handleArchiveThread)
	mux.HandleFunc("POST /inbox/chats/{id}/star", s.handleStarThread)
	mux.HandleFunc("POST /inbox/tasks/{id}/archive", s.handleArchiveTask)
	mux.HandleFunc("POST /inbox/tasks/{id}/done", s.handleDoneTask)
	mux.HandleFunc("POST /inbox/tasks/{id}/star", s.handleStarTask)
	mux.HandleFunc("POST /inbox/workflows/{id}/archive", s.handleArchiveThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/star", s.handleStarThread)
	mux.HandleFunc("POST /tasks", s.handleCreateTask)
	return logging.Middleware(s.log, mux)
}

func (s *Server) handleArchiveTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
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

func (s *Server) handleArchiveThread(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	state := db.ThreadStateArchived
	if thread.State == db.ThreadStateArchived {
		state = db.ThreadStateOpen
	}
	if err := s.chat.SetThreadState(id, state); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.renderPage(w, r, "chats", ui.InboxSelection{ID: id, Kind: "chat"})
}

func (s *Server) handleChats(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "chats", ui.InboxSelection{})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	title := strings.TrimSpace(r.FormValue("title"))
	if title != "" {
		if _, err := s.tasks.Create(title); err != nil {
			s.fail(w, err)
			return
		}
	}
	http.Redirect(w, r, "/tasks", http.StatusSeeOther)
}

func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	mode, prompt = resolveComposeMode(mode, prompt, r.FormValue("view"))
	createOnly := r.FormValue("create_only") == "true"
	if prompt == "" && !createOnly {
		s.redirectBack(w, r)
		return
	}
	switch mode {
	case "task":
		if prompt == "" {
			prompt = "New task"
		}
		task, err := s.tasks.Create(prompt)
		if err != nil {
			s.fail(w, err)
			return
		}
		http.Redirect(w, r, "/inbox/tasks/"+strconv.FormatInt(task.ID, 10), http.StatusSeeOther)
	case "workflow":
		thread, err := s.chat.CreateThread("contact", createTitle(prompt, "New workflow", createOnly))
		if err != nil {
			s.fail(w, err)
			return
		}
		if prompt != "" && !createOnly {
			if _, err := s.chat.Send(thread.ID, prompt); err != nil {
				s.fail(w, err)
				return
			}
		}
		http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(thread.ID, 10), http.StatusSeeOther)
	default:
		thread, err := s.chat.CreateThread("", createTitle(prompt, "New chat", createOnly))
		if err != nil {
			s.fail(w, err)
			return
		}
		if prompt != "" && !createOnly {
			if _, err := s.chat.Send(thread.ID, prompt); err != nil {
				s.fail(w, err)
				return
			}
		}
		http.Redirect(w, r, "/inbox/chats/"+strconv.FormatInt(thread.ID, 10), http.StatusSeeOther)
	}
}

func (s *Server) handleDoneTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	task, err := s.tasks.Get(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if _, err := s.tasks.SetCompleted(id, !task.Completed); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleInbox(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "inbox", ui.InboxSelection{})
}

func (s *Server) handleStarTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	task, err := s.tasks.Get(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if _, err := s.tasks.SetStarred(id, !task.Starred); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleStarThread(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if err := s.chat.SetThreadStarred(id, !thread.Starred); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleStarred(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "starred", ui.InboxSelection{})
}

func (s *Server) handleTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.renderPage(w, r, "tasks", ui.InboxSelection{ID: id, Kind: "task"})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "tasks", ui.InboxSelection{})
}

func (s *Server) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	s.renderPage(w, r, "workflows", ui.InboxSelection{ID: id, Kind: "workflow"})
}

func (s *Server) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "workflows", ui.InboxSelection{})
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	httperr.Log(s.log, "inbox error", err)
	httperr.Error(w, err, http.StatusInternalServerError)
}

func (s *Server) notFoundOr(w http.ResponseWriter, err error) {
	if db.IsTaskNotFound(err) || db.IsThreadNotFound(err) {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	s.fail(w, err)
}

func (s *Server) redirectBack(w http.ResponseWriter, r *http.Request) {
	back := r.FormValue("back")
	if back == "" {
		back = r.Header.Get("Referer")
	}
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) renderPage(w http.ResponseWriter, r *http.Request, view string, selected ui.InboxSelection) {
	props, err := s.props(view, selected)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.InboxPage(props))
}

func (s *Server) props(view string, selected ui.InboxSelection) (ui.InboxProps, error) {
	items, counts, err := s.items(view)
	if err != nil {
		return ui.InboxProps{}, err
	}
	props := ui.InboxProps{
		Counts: counts,
		Items:  items,
		View:   view,
	}
	props.Selected = selected
	if selected.Kind == "" {
		return props, nil
	}
	switch selected.Kind {
	case "task":
		detail, err := s.taskDetail(selected.ID)
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Task = &detail
	case "chat", "workflow":
		detail, err := s.chatDetail(selected.ID, selected.Kind)
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Thread = &detail
	}
	return props, nil
}

func (s *Server) items(view string) ([]ui.InboxItem, ui.InboxCounts, error) {
	tasks, err := s.tasks.All()
	if err != nil {
		return nil, ui.InboxCounts{}, errors.Wrap(err, "list tasks")
	}
	threads, err := s.chat.Threads()
	if err != nil {
		return nil, ui.InboxCounts{}, errors.Wrap(err, "list threads")
	}
	counts := ui.InboxCounts{}
	all := make([]ui.InboxItem, 0, len(tasks)+len(threads))
	for _, task := range tasks {
		item := taskItem(task)
		addCounts(&counts, item)
		if includeItem(view, item) {
			all = append(all, item)
		}
	}
	for _, thread := range threads {
		kind := "chat"
		agent := s.chat.AgentName(thread)
		if isWorkflowAgent(agent) {
			kind = "workflow"
		}
		item := threadItem(thread, kind, agent)
		addCounts(&counts, item)
		if includeItem(view, item) {
			all = append(all, item)
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		return all[i].UpdatedAt.After(all[j].UpdatedAt)
	})
	return all, counts, nil
}

func (s *Server) chatDetail(id int64, kind string) (ui.InboxThreadDetail, error) {
	view, err := s.chat.View(id)
	if err != nil {
		return ui.InboxThreadDetail{}, err
	}
	title := view.Thread.Title
	if title == "" {
		title = "New chat"
	}
	messages := make([]ui.ChatMessageProps, 0, len(view.Messages))
	for _, m := range view.Messages {
		messages = append(messages, ui.ChatMessageProps{
			Author: m.Author,
			Body:   m.Body,
			ID:     m.ID,
			Role:   m.Role,
			Status: m.Status,
			Tools:  view.ToolCalls[m.ID],
		})
	}
	agent := s.chat.AgentName(view.Thread)
	return ui.InboxThreadDetail{
		Agent:    agent,
		Archived: view.Thread.State == db.ThreadStateArchived,
		ID:       id,
		Kind:     kind,
		Messages: messages,
		Starred:  view.Thread.Starred,
		Title:    title,
	}, nil
}

func (s *Server) taskDetail(id int64) (ui.InboxTaskDetail, error) {
	view, err := s.tasks.ViewTask(todo.FilterAll, id)
	if err != nil {
		return ui.InboxTaskDetail{}, err
	}
	if view.Detail == nil {
		return ui.InboxTaskDetail{}, db.ErrTaskNotFound
	}
	return ui.InboxTaskDetail{
		Notes:    view.Detail.Notes,
		Subitems: view.Detail.Subitems,
		Task:     view.Detail.Task,
	}, nil
}

func addCounts(counts *ui.InboxCounts, item ui.InboxItem) {
	if !item.Archived {
		counts.Inbox++
	}
	if item.Starred && !item.Archived {
		counts.Starred++
	}
	switch item.Kind {
	case "chat":
		counts.Chats++
	case "task":
		counts.Tasks++
	case "workflow":
		counts.Workflows++
	}
}

func includeItem(view string, item ui.InboxItem) bool {
	switch view {
	case "chats":
		return item.Kind == "chat"
	case "starred":
		return item.Starred && !item.Archived
	case "tasks":
		return item.Kind == "task"
	case "workflows":
		return item.Kind == "workflow"
	default:
		return !item.Archived
	}
}

func isWorkflowAgent(agent string) bool {
	return agent == "contact" || strings.HasPrefix(agent, "workflow")
}

func createTitle(prompt, fallback string, createOnly bool) string {
	if createOnly || prompt == "" {
		return fallback
	}
	return ""
}

func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func resolveComposeMode(mode, prompt, view string) (string, string) {
	if mode == "auto" || mode == "" {
		for _, prefix := range []string{"chat", "task", "workflow"} {
			if rest, ok := strings.CutPrefix(strings.ToLower(prompt), prefix+":"); ok {
				return prefix, strings.TrimSpace(prompt[len(prompt)-len(rest):])
			}
		}
		switch view {
		case "tasks":
			return "task", prompt
		case "workflows":
			return "workflow", prompt
		default:
			return "chat", prompt
		}
	}
	return mode, prompt
}

func taskItem(task db.Task) ui.InboxItem {
	status := "Active"
	if task.Completed {
		status = "Done"
	}
	return ui.InboxItem{
		Archived:  task.Archived,
		Done:      task.Completed,
		From:      "Task",
		Href:      "/inbox/tasks/" + strconv.FormatInt(task.ID, 10),
		ID:        task.ID,
		Kind:      "task",
		Snippet:   status,
		Starred:   task.Starred,
		Title:     task.Title,
		UpdatedAt: task.UpdatedAt.Time,
		When:      when(task.UpdatedAt.Time),
		Workflow:  false,
	}
}

func threadItem(thread db.Thread, kind, agent string) ui.InboxItem {
	title := thread.Title
	if title == "" {
		title = "New chat"
	}
	return ui.InboxItem{
		Archived:  thread.State == db.ThreadStateArchived,
		Done:      thread.State == db.ThreadStateResolved,
		From:      titleKind(kind, agent),
		Href:      "/inbox/" + titleKindPath(kind) + "/" + strconv.FormatInt(thread.ID, 10),
		ID:        thread.ID,
		Kind:      kind,
		Snippet:   agent,
		Starred:   thread.Starred,
		Title:     title,
		UpdatedAt: thread.UpdatedAt.Time,
		When:      when(thread.UpdatedAt.Time),
		Workflow:  kind == "workflow",
	}
}

func titleKind(kind, agent string) string {
	if kind == "workflow" {
		return "Workflow"
	}
	if agent != "" {
		return "Chat"
	}
	return "Thread"
}

func titleKindPath(kind string) string {
	if kind == "workflow" {
		return "workflows"
	}
	if kind == "task" {
		return "tasks"
	}
	return "chats"
}

func when(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	now := time.Now()
	if t.Year() == now.Year() && t.YearDay() == now.YearDay() {
		return t.Local().Format("3:04 PM")
	}
	if t.Year() == now.Year() {
		return t.Local().Format("Jan 2")
	}
	return t.Local().Format("2006-01-02")
}
