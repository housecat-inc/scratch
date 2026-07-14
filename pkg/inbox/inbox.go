package inbox

import (
	"encoding/json"
	"fmt"
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
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/server/logging"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
)

type Server struct {
	chat  *chat.Service
	flows *flow.Engine
	log   *slog.Logger
	tasks *todo.Service
}

func NewServer(tasks *todo.Service, chat *chat.Service, flows *flow.Engine, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{chat: chat, flows: flows, log: log, tasks: tasks}
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
	mux.HandleFunc("GET /inbox/schedules/{name}", s.handleSchedule)
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
	mux.HandleFunc("POST /inbox/schedules/{name}/pause", s.handlePauseSchedule)
	mux.HandleFunc("POST /inbox/schedules/{name}/resume", s.handleResumeSchedule)
	mux.HandleFunc("POST /inbox/schedules/{name}/trigger", s.handleTriggerSchedule)
	mux.HandleFunc("POST /inbox/workflows/{id}/archive", s.handleArchiveThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/edit", s.handleEditWorkflow)
	mux.HandleFunc("POST /inbox/workflows/{id}/fork", s.handleForkWorkflow)
	mux.HandleFunc("POST /inbox/workflows/{id}/resolve", s.handleResolveWorkflow)
	mux.HandleFunc("POST /inbox/workflows/{id}/star", s.handleStarThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/stop", s.handleStopThread)
	mux.HandleFunc("POST /inbox/workflows/{id}/trash", s.handleTrashThread)
	mux.HandleFunc("POST /webhooks/{id}", s.handleWebhook)
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
	if providerModel := r.URL.Query().Get("provider_model"); providerModel != "" {
		agent, model = chat.ParseProviderModel(providerModel)
		agent = chatAgent(agent, s.chat.AgentNames())
		model = chatModel(agent, model)
	}
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
	if providerModel := r.FormValue("provider_model"); providerModel != "" {
		agent, model = chat.ParseProviderModel(providerModel)
		agent = chatAgent(agent, s.chat.AgentNames())
		model = chatModel(agent, model)
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	mode := resolveComposeMode(strings.TrimSpace(r.FormValue("mode")))
	createOnly := r.FormValue("create_only") == "true"
	hasFiles := composeHasFiles(r)
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
		workflowType := workflowAgent(r.FormValue("workflow_type"))
		thread, workflowID, err := s.chat.CreateWorkflowThread(workflowType, workflowTitle(workflowType))
		if err != nil {
			s.fail(w, err)
			return
		}
		if err := s.flows.Start(workflowType, workflowID); err != nil {
			s.fail(w, err)
			return
		}
		s.flows.Await(workflowID, 2*time.Second)
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

func (s *Server) handleSchedule(w http.ResponseWriter, r *http.Request) {
	s.renderPage(w, r, "workflows", ui.InboxSelection{Kind: "schedule", Name: r.PathValue("name")})
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
	if db.IsTaskNotFound(err) || db.IsThreadNotFound(err) || errors.Is(err, flow.ErrScheduleNotFound) {
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
		ChatOptions:   chat.ProviderModelOptions(s.chat.AgentNames()),
		Counts:        counts,
		Items:         items,
		View:          view,
	}
	if view == "workflows" {
		schedules, err := s.schedules()
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Schedules = schedules
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
	case "chat":
		detail, err := s.chatDetail(selected.ID, selected.Kind)
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Thread = &detail
	case "workflow":
		detail, err := s.workflowDetail(selected.ID)
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Workflow = &detail
	case "schedule":
		detail, err := s.scheduleDetail(selected.Name)
		if err != nil {
			return ui.InboxProps{}, err
		}
		props.Schedule = &detail
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
		if workflowID := s.chat.ThreadWorkflowID(thread); workflowID != "" {
			item := threadItem(thread, "workflow", "", s.workflowSnippet(workflowID))
			addCounts(&counts, item)
			if includeItem(view, filter, item) {
				all = append(all, item)
			}
			continue
		}
		agent := s.chat.AgentName(thread)
		prompt, err := s.threadPrompt(thread.ID)
		if err != nil {
			return nil, ui.InboxCounts{}, errors.Wrap(err, "first thread prompt")
		}
		item := threadItem(thread, "chat", agent, prompt)
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
	prompt, err := s.threadPrompt(id)
	if err != nil {
		return ui.InboxThreadDetail{}, errors.Wrap(err, "thread prompt")
	}
	return ui.InboxThreadDetail{
		Access:      s.chat.ThreadAccess(view.Thread),
		Agent:       agent,
		Archived:    view.Thread.State == db.ThreadStateArchived,
		Description: chat.FriendlyDescription(prompt),
		ID:          id,
		Kind:        kind,
		Messages:    messages,
		Starred:     view.Thread.Starred,
		Streaming:   view.Streaming,
		Title:       title,
	}, nil
}

func (s *Server) workflowDetail(id int64) (ui.InboxWorkflowDetail, error) {
	thread, err := s.chat.Thread(id)
	if err != nil {
		return ui.InboxWorkflowDetail{}, err
	}
	workflowID := s.chat.ThreadWorkflowID(thread)
	run, err := s.flows.Run(workflowID)
	if errors.Is(err, flow.ErrRunNotFound) {
		run = flow.RunView{ID: workflowID, Status: "PENDING"}
	} else if err != nil {
		return ui.InboxWorkflowDetail{}, err
	}
	title := thread.Title
	if title == "" {
		title = "New workflow"
	}
	detail := ui.InboxWorkflowDetail{
		Archived: thread.State == db.ThreadStateArchived,
		Awaiting: run.Blocked,
		ID:       id,
		Running:  run.Running(),
		Starred:  thread.Starred,
		Status:   run.Status,
		Title:    title,
	}
	for _, step := range run.Steps {
		detail.Items = append(detail.Items, s.workflowItemProps(id, step))
	}
	return detail, nil
}

func (s *Server) scheduleDetail(name string) (ui.InboxScheduleDetail, error) {
	schedules, err := s.schedules()
	if err != nil {
		return ui.InboxScheduleDetail{}, err
	}
	var schedule ui.WorkflowScheduleView
	found := false
	for _, sc := range schedules {
		if sc.Name == name {
			schedule = sc
			found = true
			break
		}
	}
	if !found {
		return ui.InboxScheduleDetail{}, flow.ErrScheduleNotFound
	}
	runs, err := s.flows.ScheduleRuns(name)
	if err != nil {
		return ui.InboxScheduleDetail{}, err
	}
	detail := ui.InboxScheduleDetail{
		Cron:      schedule.Cron,
		LastFired: schedule.LastFired,
		Name:      schedule.Name,
		Paused:    schedule.Paused,
		Status:    schedule.Status,
	}
	for _, run := range runs {
		item := ui.ScheduleRunProps{
			CreatedAt: run.CreatedAt,
			ID:        run.ID,
			Status:    run.Status,
		}
		for _, step := range run.Steps {
			item.Items = append(item.Items, s.workflowItemProps(0, step))
		}
		detail.Runs = append(detail.Runs, item)
	}
	return detail, nil
}

func (s *Server) workflowItemProps(threadID int64, step flow.StepView) ui.WorkflowItemProps {
	item := ui.WorkflowItemProps{
		Answer:   step.Answer,
		Detail:   step.Detail,
		Duration: step.Duration,
		Failed:   step.Failed,
		Input:    step.Input,
		Kind:     step.Kind,
		Running:  step.Status == flow.StepRunning,
		Summary:  step.Summary,
		Title:    step.Title,
	}
	if threadID == 0 || step.Kind != flow.KindForm || step.Form == nil {
		return item
	}
	form := chat.FormProps(fmt.Sprintf("/inbox/workflows/%d/resolve", threadID), *step.Form)
	form.HideMessage = true
	if step.Pending {
		form.Plain = true
	} else {
		form.Action = fmt.Sprintf("/inbox/workflows/%d/edit", threadID)
		form.Editable = true
		form.ForkAction = fmt.Sprintf("/inbox/workflows/%d/fork", threadID)
		for i := range form.Fields {
			if v, ok := step.Values[form.Fields[i].Name]; ok {
				form.Fields[i].Value = v
			}
		}
	}
	item.Form = &form
	return item
}

func (s *Server) schedules() ([]ui.WorkflowScheduleView, error) {
	schedules, err := s.flows.Schedules()
	if err != nil {
		return nil, err
	}
	views := make([]ui.WorkflowScheduleView, 0, len(schedules))
	for _, sc := range schedules {
		views = append(views, ui.WorkflowScheduleView{
			Cron:      sc.Cron,
			LastFired: scheduleLastFired(sc.LastFiredAt),
			Name:      sc.Name,
			Paused:    sc.Paused,
			Status:    sc.Status,
		})
	}
	return views, nil
}

func (s *Server) workflowSnippet(workflowID string) string {
	status, err := s.flows.Status(workflowID)
	if err != nil {
		return ""
	}
	switch status {
	case "SUCCESS":
		return "Completed"
	case "ERROR":
		return "Failed"
	case "CANCELLED":
		return "Cancelled"
	default:
		return "Running"
	}
}

func (s *Server) handleResolveWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	workflowID := s.chat.ThreadWorkflowID(thread)
	if workflowID == "" {
		http.Error(w, "not a workflow", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	action := r.PostForm.Get("action")
	elicitationID := r.PostForm.Get("elicitation_id")
	values := formValues(r)
	err = s.flows.Resolve(workflowID, elicitationID, action, values)
	switch {
	case err == nil, errors.Is(err, flow.ErrFormResolved):
		http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, flow.ErrFormStale):
		if !s.recoverStaleWorkflowForm(w, id, workflowID, elicitationID, action, values) {
			return
		}
		http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
	default:
		s.fail(w, err)
	}
}

func (s *Server) recoverStaleWorkflowForm(w http.ResponseWriter, threadID int64, workflowID, elicitationID, action string, values map[string]string) bool {
	forkedID, err := s.flows.EditForm(workflowID, elicitationID, action, values)
	switch {
	case err == nil:
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return false
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
		return false
	default:
		s.fail(w, err)
		return false
	}
	if err := s.flows.Cancel(workflowID); err != nil {
		s.log.Warn("cancel stale workflow", "id", workflowID, "err", err)
	}
	if err := s.chat.SetThreadWorkflowID(threadID, forkedID); err != nil {
		s.fail(w, err)
		return false
	}
	return true
}

func (s *Server) handleEditWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	workflowID := s.chat.ThreadWorkflowID(thread)
	if workflowID == "" {
		http.Error(w, "not a workflow", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	forkedID, err := s.flows.EditForm(workflowID, r.PostForm.Get("elicitation_id"), elicit.ActionAccept, formValues(r))
	switch {
	case err == nil:
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
		return
	default:
		s.fail(w, err)
		return
	}
	if err := s.flows.Cancel(workflowID); err != nil {
		s.log.Warn("cancel superseded workflow", "id", workflowID, "err", err)
	}
	if err := s.chat.SetThreadWorkflowID(id, forkedID); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleForkWorkflow(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	workflowID := s.chat.ThreadWorkflowID(thread)
	if workflowID == "" {
		http.Error(w, "not a workflow", http.StatusNotFound)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	forkedID, err := s.flows.Fork(workflowID, r.PostForm.Get("elicitation_id"))
	switch {
	case err == nil:
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
		return
	default:
		s.fail(w, err)
		return
	}
	forked, err := s.chat.CreateForkedWorkflowThread(s.chat.WorkflowName(thread), forkedID, forkTitle(thread.Title))
	if err != nil {
		s.fail(w, err)
		return
	}
	s.flows.Await(forkedID, 2*time.Second)
	http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(forked.ID, 10), http.StatusSeeOther)
}

func (s *Server) handleWebhook(w http.ResponseWriter, r *http.Request) {
	var payload flow.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if err := s.flows.DeliverWebhook(r.PathValue("id"), payload, r.Header.Get("Idempotency-Key")); err != nil {
		if errors.Is(err, flow.ErrRunNotFound) {
			http.Error(w, "workflow not found", http.StatusNotFound)
			return
		}
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusAccepted)
}

func (s *Server) handlePauseSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.flows.PauseSchedule(name); err != nil {
		s.fail(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleResumeSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if err := s.flows.ResumeSchedule(name); err != nil {
		s.fail(w, err)
		return
	}
	s.redirectBack(w, r)
}

func (s *Server) handleTriggerSchedule(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	if _, err := s.flows.TriggerSchedule(name); err != nil {
		s.fail(w, err)
		return
	}
	s.redirectBack(w, r)
}

func formValues(r *http.Request) map[string]string {
	values := map[string]string{}
	for key := range r.PostForm {
		if name, ok := strings.CutPrefix(key, "f_"); ok {
			values[name] = r.PostForm.Get(key)
		}
	}
	return values
}

func forkTitle(title string) string {
	if strings.TrimSpace(title) == "" {
		title = "Workflow"
	}
	return title + " (fork)"
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

func chatAgent(agent string, agents []string) string {
	agent = strings.TrimSpace(agent)
	if agent == "" {
		return ""
	}
	for _, known := range agents {
		if agent == known {
			return agent
		}
	}
	return ""
}

func chatAgentOptions(agents []string) []string {
	out := make([]string, 0, len(agents))
	for _, agent := range agents {
		if agent == "" {
			continue
		}
		out = append(out, agent)
	}
	return out
}

func chatModel(agent, model string) string {
	return chat.NormalizeModel(agent, model)
}

func workflowAgent(typ string) string {
	switch strings.TrimSpace(typ) {
	case "countdown":
		return "countdown"
	case "create-pr":
		return "create-pr"
	case "deploy":
		return "deploy"
	case "fan-out":
		return "fan-out"
	case "stream":
		return "stream"
	case "update-claude":
		return "update-claude"
	case "webhook":
		return "webhook"
	default:
		return "greet"
	}
}

func workflowTitle(typ string) string {
	switch typ {
	case "countdown":
		return "Countdown"
	case "create-pr":
		return "Create pull request"
	case "deploy":
		return "Deploy"
	case "fan-out":
		return "Parallel jobs"
	case "stream":
		return "Log stream"
	case "update-claude":
		return "Update Claude Code"
	case "webhook":
		return "Webhook"
	default:
		return "Greet"
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

func resolveComposeMode(mode string) string {
	switch mode {
	case "task", "workflow":
		return mode
	default:
		return "chat"
	}
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
		Snippet:   chat.FriendlyDescription(prompt),
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

func scheduleLastFired(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return "last at " + when(t)
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
