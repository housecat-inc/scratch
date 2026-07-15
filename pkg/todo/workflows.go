package todo

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

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
	s.render(w, r, ui.WorkflowsPage(ui.WorkflowsProps{Counts: s.navCounts(), Runs: runs}))
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
	if r.Header.Get("HX-Request") != "" {
		s.render(w, r, ui.WorkflowRunBody(detail))
		return
	}
	s.render(w, r, ui.WorkflowRunPage(ui.WorkflowRunProps{Counts: s.navCounts(), Detail: detail}))
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
	err = s.flows.Resolve(workflowID, r.PostForm.Get("elicitation_id"), r.PostForm.Get("action"), values)
	switch {
	case err == nil, errors.Is(err, flow.ErrFormResolved):
		http.Redirect(w, r, "/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, flow.ErrFormNotFound):
		http.Error(w, "form not found", http.StatusNotFound)
	default:
		s.fail(w, err)
	}
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
