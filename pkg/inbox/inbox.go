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
	mux.HandleFunc("GET /inbox", s.handleInbox)
	mux.HandleFunc("GET /inbox/chats", s.handleChats)
	mux.HandleFunc("GET /inbox/chats/new", s.handleNewChat)
	mux.HandleFunc("GET /inbox/tasks", s.handleTasks)
	mux.HandleFunc("GET /inbox/workflows", s.handleWorkflows)
	mux.HandleFunc("GET /starred", s.handleStarred)
	mux.HandleFunc("GET /inbox/chats/{id}", s.handleChat)
	mux.HandleFunc("GET /inbox/tasks/{id}", s.handleTask)
	mux.HandleFunc("GET /inbox/workflows/{id}", s.handleWorkflow)
	mux.HandleFunc("POST /compose", s.handleCompose)
	mux.HandleFunc("POST /inbox/chats/{id}/archive", s.handleArchiveThread)
	mux.HandleFunc("POST /inbox/chats/{id}/star", s.handleStarThread)
	mux.HandleFunc("POST /inbox/chats/{id}/stop", s.handleStopThread)
	mux.HandleFunc("POST /inbox/chats/{id}/trash", s.handleTrashThread)
	mux.HandleFunc("POST /inbox/tasks/{id}/archive", s.handleArchiveTask)
	mux.HandleFunc("POST /inbox/tasks/{id}/done", s.handleDoneTask)
	mux.HandleFunc("POST /inbox/tasks/{id}/star", s.handleStarTask)
	mux.HandleFunc("POST /inbox/tasks/{id}/trash", s.handleTrashTask)
	mux.HandleFunc("POST /inbox/tasks/{id}", s.handleUpdateTask)
	mux.HandleFunc("POST /inbox/workflows/{id}/archive", s.handleArchiveThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/star", s.handleStarThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/stop", s.handleStopThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/trash", s.handleTrashThread)
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

func (s *Server) handleNewChat(w http.ResponseWriter, r *http.Request) {
	agent := chatAgent(r.URL.Query().Get("agent"), s.chat.AgentNames())
	model := chatModel(agent, r.URL.Query().Get("model"))
	props, err := s.props("chats", archiveFilter("chats", r.URL.Query().Get("archived")), ui.InboxSelection{})
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	props.Draft = &ui.InboxDraftDetail{
		Agent: agent,
		Model: model,
		Title: "New chat",
	}
	s.render(w, r, ui.InboxPage(props))
}

func (s *Server) handleCompose(w http.ResponseWriter, r *http.Request) {
	agent := chatAgent(r.FormValue("agent"), s.chat.AgentNames())
	model := chatModel(agent, r.FormValue("model"))
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	mode := strings.TrimSpace(r.FormValue("mode"))
	createOnly := r.FormValue("create_only") == "true"
	hasFiles := composeHasFiles(r)
	mode, prompt = resolveComposeMode(mode, prompt, r.FormValue("view"), hasFiles)
	if prompt == "" && !createOnly && !hasFiles {
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
		thread, err := s.chat.CreateThread(workflowAgent(r.FormValue("workflow_type")), createTitle(prompt, "New workflow", createOnly))
		if err != nil {
			s.fail(w, err)
			return
		}
		attachmentIDs, err := s.composeAttachments(r, thread.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if (prompt != "" || len(attachmentIDs) > 0) && !createOnly {
			if prompt == "" {
				prompt = "See attached files."
			}
			if _, err := s.chat.Send(thread.ID, prompt, attachmentIDs...); err != nil {
				s.fail(w, err)
				return
			}
		}
		http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(thread.ID, 10), http.StatusSeeOther)
	default:
		thread, err := s.chat.CreateThreadWithModel(agent, model, createTitle(prompt, "New chat", createOnly))
		if err != nil {
			s.fail(w, err)
			return
		}
		attachmentIDs, err := s.composeAttachments(r, thread.ID)
		if err != nil {
			s.fail(w, err)
			return
		}
		if (prompt != "" || len(attachmentIDs) > 0) && !createOnly {
			if prompt == "" {
				prompt = "See attached files."
			}
			if _, err := s.chat.Send(thread.ID, prompt, attachmentIDs...); err != nil {
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

func (s *Server) handleStopThread(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.chat.StopThread(id); err != nil && !chat.IsThreadNotBusy(err) {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
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

func (s *Server) handleTrashTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.tasks.Delete(id); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleTrashThread(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := s.chat.DeleteThread(id); err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, err)
		return
	}
	if _, ok := r.Form["title"]; ok {
		title := strings.TrimSpace(r.FormValue("title"))
		if title != "" {
			if _, err := s.tasks.Edit(id, title); err != nil {
				s.notFoundOr(w, err)
				return
			}
		}
	}
	if _, ok := r.Form["description"]; ok {
		if _, err := s.tasks.EditDescription(id, strings.TrimSpace(r.FormValue("description"))); err != nil {
			s.notFoundOr(w, err)
			return
		}
	}
	http.Redirect(w, r, "/inbox/tasks/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
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

func composeHasFiles(r *http.Request) bool {
	if r.MultipartForm == nil {
		return false
	}
	for _, files := range r.MultipartForm.File {
		if len(files) > 0 {
			return true
		}
	}
	return false
}

func (s *Server) composeAttachments(r *http.Request, threadID int64) ([]int64, error) {
	if r.MultipartForm == nil {
		return nil, nil
	}
	headers := r.MultipartForm.File["file"]
	ids := make([]int64, 0, len(headers))
	for _, header := range headers {
		file, err := header.Open()
		if err != nil {
			return nil, errors.Wrap(err, "open compose attachment")
		}
		attachment, err := s.chat.AddAttachment(threadID, header.Filename, header.Header.Get("Content-Type"), file)
		closeErr := file.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, errors.Wrap(closeErr, "close compose attachment")
		}
		ids = append(ids, attachment.ID)
	}
	return ids, nil
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
	props, err := s.props(view, archiveFilter(view, r.URL.Query().Get("archived")), selected)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.InboxPage(props))
}

func (s *Server) props(view, filter string, selected ui.InboxSelection) (ui.InboxProps, error) {
	items, counts, err := s.items(view, filter)
	if err != nil {
		return ui.InboxProps{}, err
	}
	props := ui.InboxProps{
		ArchiveFilter: filter,
		Agents:        chatAgentOptions(s.chat.AgentNames()),
		Counts:        counts,
		Items:         items,
		View:          view,
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

func (s *Server) items(view, filter string) ([]ui.InboxItem, ui.InboxCounts, error) {
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
		if includeItem(view, filter, item) {
			all = append(all, item)
		}
	}
	for _, thread := range threads {
		kind := "chat"
		agent := s.chat.AgentName(thread)
		if isWorkflowAgent(agent) {
			kind = "workflow"
		}
		prompt, err := s.threadPrompt(thread.ID)
		if err != nil {
			return nil, ui.InboxCounts{}, errors.Wrap(err, "first thread prompt")
		}
		item := threadItem(thread, kind, agent, prompt)
		addCounts(&counts, item)
		if includeItem(view, filter, item) {
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
		messages = append(messages, chat.MessageProps(view, m))
	}
	agent := s.chat.AgentName(view.Thread)
	return ui.InboxThreadDetail{
		Access:    s.chat.ThreadAccess(view.Thread),
		Agent:     agent,
		Archived:  view.Thread.State == db.ThreadStateArchived,
		ID:        id,
		Kind:      kind,
		Messages:  messages,
		Starred:   view.Thread.Starred,
		Streaming: view.Streaming,
		Title:     title,
	}, nil
}

func (s *Server) taskDetail(id int64) (ui.InboxTaskDetail, error) {
	task, err := s.tasks.Get(id)
	if err != nil {
		return ui.InboxTaskDetail{}, err
	}
	return ui.InboxTaskDetail{
		Task: task,
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

func archiveFilter(view, filter string) string {
	if view != "chats" && view != "tasks" && view != "workflows" {
		return ""
	}
	switch filter {
	case "active", "archived":
		return filter
	default:
		return "all"
	}
}

func includeItem(view, filter string, item ui.InboxItem) bool {
	switch view {
	case "chats":
		return item.Kind == "chat" && includeArchiveFilter(filter, item)
	case "starred":
		return item.Starred && !item.Archived
	case "tasks":
		return item.Kind == "task" && includeTaskFilter(filter, item)
	case "workflows":
		return item.Kind == "workflow" && includeArchiveFilter(filter, item)
	default:
		return !item.Archived
	}
}

func includeArchiveFilter(filter string, item ui.InboxItem) bool {
	switch filter {
	case "active":
		return !item.Archived
	case "archived":
		return item.Archived
	default:
		return true
	}
}

func includeTaskFilter(filter string, item ui.InboxItem) bool {
	switch filter {
	case "active":
		return !item.Archived && !item.Done
	case "archived":
		return item.Archived
	default:
		return true
	}
}

func isWorkflowAgent(agent string) bool {
	return agent == "contact" || strings.HasPrefix(agent, "workflow")
}

func chatAgent(agent string, agents []string) string {
	agent = strings.TrimSpace(agent)
	if agent == "" || isWorkflowAgent(agent) {
		return ""
	}
	for _, known := range agents {
		if agent == known && !isWorkflowAgent(known) {
			return agent
		}
	}
	return ""
}

func chatAgentOptions(agents []string) []string {
	out := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent == "" || isWorkflowAgent(agent) {
			continue
		}
		out = append(out, agent)
	}
	return out
}

func chatModel(agent, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == "default" {
		return ""
	}
	switch agent {
	case "claude":
		switch model {
		case "haiku", "opus", "sonnet":
			return model
		}
	case "codex":
		switch model {
		case "gpt-5", "gpt-5.1", "gpt-5.5":
			return model
		}
	}
	return ""
}

func workflowAgent(typ string) string {
	switch strings.TrimSpace(typ) {
	case "", "contact":
		return "contact"
	default:
		return "contact"
	}
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

func resolveComposeMode(mode, prompt, view string, hasFiles bool) (string, string) {
	if mode == "auto" || mode == "" {
		for _, prefix := range []string{"chat", "task", "workflow"} {
			if rest, ok := strings.CutPrefix(strings.ToLower(prompt), prefix+":"); ok {
				return prefix, strings.TrimSpace(prompt[len(prompt)-len(rest):])
			}
		}
		if hasFiles {
			return "chat", prompt
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
	return ui.InboxItem{
		Archived:  task.Archived,
		Done:      task.Completed,
		From:      "Task",
		Href:      "/inbox/tasks/" + strconv.FormatInt(task.ID, 10),
		ID:        task.ID,
		Kind:      "task",
		Snippet:   task.Description,
		Starred:   task.Starred,
		Title:     task.Title,
		UpdatedAt: task.UpdatedAt.Time,
		When:      when(task.UpdatedAt.Time),
		Workflow:  false,
	}
}

func threadItem(thread db.Thread, kind, agent, prompt string) ui.InboxItem {
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
		Snippet:   prompt,
		Starred:   thread.Starred,
		Title:     title,
		UpdatedAt: thread.UpdatedAt.Time,
		When:      when(thread.UpdatedAt.Time),
		Workflow:  kind == "workflow",
	}
}

func (s *Server) threadPrompt(id int64) (string, error) {
	return s.chat.ThreadPrompt(id)
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
