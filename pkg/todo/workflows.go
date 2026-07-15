package todo

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/ui"
)

func workflowStartAllowed(workflowType string) bool {
	for _, opt := range ui.TaskWorkflowOptions() {
		if opt.Type == workflowType {
			return true
		}
	}
	return false
}

func (s *WebServer) workflowsEnabled() bool {
	return s.flows != nil && s.chat != nil
}

func (s *WebServer) workflowThreads() []db.Thread {
	if s.chat == nil {
		return nil
	}
	threads, err := s.chat.Threads()
	if err != nil {
		return nil
	}
	out := make([]db.Thread, 0, len(threads))
	for _, thread := range threads {
		if s.chat.ThreadWorkflowID(thread) != "" && thread.State != db.ThreadStateArchived {
			out = append(out, thread)
		}
	}
	return out
}

func (s *WebServer) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if !s.workflowsEnabled() {
		http.NotFound(w, r)
		return
	}
	threads := s.workflowThreads()
	runs := make([]ui.WorkflowRunItem, 0, len(threads))
	for _, thread := range threads {
		status, _ := s.flows.Status(s.chat.ThreadWorkflowID(thread))
		title := thread.Title
		if title == "" {
			title = "New workflow"
		}
		runs = append(runs, ui.WorkflowRunItem{
			Href:   "/workflows/" + strconv.FormatInt(thread.ID, 10),
			ID:     thread.ID,
			Status: status,
			Title:  title,
			When:   todoWhen(thread.UpdatedAt.Time),
		})
	}
	chatOptions, chatLabel := s.chatActions()
	s.render(w, r, ui.WorkflowsPage(ui.WorkflowsProps{
		ChatLabel:   chatLabel,
		ChatOptions: chatOptions,
		Counts:      s.navCounts(),
		Runs:        runs,
	}))
}

func (s *WebServer) handleStartWorkflow(w http.ResponseWriter, r *http.Request) {
	if !s.workflowsEnabled() {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, err)
		return
	}
	workflowType := r.FormValue("workflow_type")
	if !workflowStartAllowed(workflowType) {
		http.Error(w, "unknown workflow", http.StatusBadRequest)
		return
	}
	thread, workflowID, err := s.chat.CreateWorkflowThread(workflowType, ui.WorkflowLabel(workflowType))
	if err != nil {
		s.fail(w, err)
		return
	}
	if err := s.flows.Start(workflowType, workflowID); err != nil {
		s.fail(w, err)
		return
	}
	s.flows.Await(workflowID, 2*time.Second)
	http.Redirect(w, r, "/workflows/"+strconv.FormatInt(thread.ID, 10), http.StatusSeeOther)
}

func (s *WebServer) handleWorkflow(w http.ResponseWriter, r *http.Request) {
	if !s.workflowsEnabled() {
		http.NotFound(w, r)
		return
	}
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	detail, err := s.workflowDetail(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	chatOptions, chatLabel := s.chatActions()
	s.render(w, r, ui.WorkflowRunPage(ui.WorkflowRunProps{
		ChatLabel:   chatLabel,
		ChatOptions: chatOptions,
		Counts:      s.navCounts(),
		Detail:      detail,
	}))
}

const workflowEventInterval = 400 * time.Millisecond

func (s *WebServer) handleWorkflowEvents(w http.ResponseWriter, r *http.Request) {
	if !s.workflowsEnabled() {
		http.NotFound(w, r)
		return
	}
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	thread, err := s.chat.Thread(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if s.chat.ThreadWorkflowID(thread) == "" {
		http.Error(w, "not a workflow", http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/event-stream")

	after, _ := strconv.Atoi(r.URL.Query().Get("after"))
	if last, err := strconv.Atoi(r.Header.Get("Last-Event-ID")); err == nil && last > after {
		after = last
	}

	ticker := time.NewTicker(workflowEventInterval)
	defer ticker.Stop()

	ctx := r.Context()
	var lastStatus, lastPending string
	for {
		detail, err := s.workflowDetail(id)
		if err != nil {
			return
		}
		if err := writeWorkflowChanged(w, ctx, "status", ui.TaskWorkflowStatus(detail), &lastStatus); err != nil {
			return
		}
		for _, item := range detail.Items {
			if !item.Durable || item.ID <= after {
				continue
			}
			if err := writeWorkflowEvent(w, ctx, "step", strconv.Itoa(item.ID), ui.WorkflowStep(item)); err != nil {
				return
			}
			after = item.ID
		}
		if err := writeWorkflowChanged(w, ctx, "pending", ui.WorkflowPending(detail), &lastPending); err != nil {
			return
		}
		flusher.Flush()
		if !detail.Running {
			fmt.Fprint(w, "event: done\ndata: done\n\n")
			flusher.Flush()
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func writeWorkflowChanged(w io.Writer, ctx context.Context, event string, comp templ.Component, last *string) error {
	var buf bytes.Buffer
	if err := comp.Render(ctx, &buf); err != nil {
		return err
	}
	if buf.String() == *last {
		return nil
	}
	*last = buf.String()
	return writeWorkflowSSE(w, event, "", buf.String())
}

func writeWorkflowEvent(w io.Writer, ctx context.Context, event, id string, comp templ.Component) error {
	var buf bytes.Buffer
	if err := comp.Render(ctx, &buf); err != nil {
		return err
	}
	return writeWorkflowSSE(w, event, id, buf.String())
}

func writeWorkflowSSE(w io.Writer, event, id, html string) error {
	if id != "" {
		if _, err := fmt.Fprintf(w, "id: %s\n", id); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "event: %s\n", event); err != nil {
		return err
	}
	for _, line := range strings.Split(strings.ReplaceAll(html, "\r", ""), "\n") {
		if _, err := fmt.Fprintf(w, "data: %s\n", line); err != nil {
			return err
		}
	}
	_, err := fmt.Fprint(w, "\n")
	return err
}

func (s *WebServer) handleResolveWorkflow(w http.ResponseWriter, r *http.Request) {
	if !s.workflowsEnabled() {
		http.NotFound(w, r)
		return
	}
	id, ok := taskPathID(w, r)
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
	values := map[string]string{}
	for key := range r.PostForm {
		if name, ok := strings.CutPrefix(key, "f_"); ok {
			values[name] = r.PostForm.Get(key)
		}
	}
	elicitationID := r.PostForm.Get("elicitation_id")
	action := r.PostForm.Get("action")
	err = s.flows.Resolve(workflowID, elicitationID, action, values)
	switch {
	case err == nil, errors.Is(err, flow.ErrFormResolved):
		http.Redirect(w, r, "/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case errors.Is(err, flow.ErrFormStale):
		if !s.recoverStaleWorkflowForm(w, id, workflowID, elicitationID, action, values) {
			return
		}
		http.Redirect(w, r, "/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
	default:
		s.fail(w, err)
	}
}

func (s *WebServer) recoverStaleWorkflowForm(w http.ResponseWriter, threadID int64, workflowID, elicitationID, action string, values map[string]string) bool {
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

func (s *WebServer) workflowDetail(id int64) (ui.InboxWorkflowDetail, error) {
	thread, err := s.chat.Thread(id)
	if err != nil {
		return ui.InboxWorkflowDetail{}, err
	}
	workflowID := s.chat.ThreadWorkflowID(thread)
	if workflowID == "" {
		return ui.InboxWorkflowDetail{}, db.ErrTaskNotFound
	}
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
		Awaiting: run.Blocked,
		ID:       id,
		Running:  run.Running(),
		Status:   run.Status,
		Title:    title,
	}
	for _, step := range run.Steps {
		detail.Items = append(detail.Items, s.workflowItemProps(id, step))
	}
	return detail, nil
}

func (s *WebServer) workflowItemProps(threadID int64, step flow.StepView) ui.WorkflowItemProps {
	item := ui.WorkflowItemProps{
		Answer:   step.Answer,
		Copy:     step.Copy,
		Detail:   step.Detail,
		Durable:  step.Durable,
		Duration: step.Duration,
		Failed:   step.Failed,
		ID:       step.ID,
		Input:    step.Input,
		Kind:     step.Kind,
		Running:  step.Status == flow.StepRunning,
		Summary:  step.Summary,
		Title:    step.Title,
	}
	if step.Kind != flow.KindForm || step.Form == nil {
		return item
	}
	form := chat.FormProps(fmt.Sprintf("/workflows/%d/resolve", threadID), *step.Form)
	form.HideMessage = true
	if step.Pending {
		form.Plain = true
	} else {
		for i := range form.Fields {
			if v, ok := step.Values[form.Fields[i].Name]; ok {
				form.Fields[i].Value = v
			}
		}
		form.Disabled = true
	}
	item.Form = &form
	return item
}
