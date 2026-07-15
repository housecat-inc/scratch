package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const (
	StepDone    = "done"
	StepFailed  = "failed"
	StepRunning = "running"

	KindForm     = "form"
	KindResponse = "response"
	KindTool     = "tool"
)

var (
	ErrFormNotFound = errors.New("form not found")
	ErrFormResolved = errors.New("form already resolved")
	ErrFormStale    = errors.New("form workflow is stale")
	ErrRunNotFound  = errors.New("workflow run not found")
)

type Deps struct {
	CountdownStep  time.Duration
	CountdownTicks int
	DBOS           dbos.DBOSContext
	Drafter        PRDrafter
	FanoutJobs     int
	Greeter        Greeter
	Log            *slog.Logger
	StageStep      time.Duration
	Updater        Updater
	WebhookTimeout time.Duration
	Workdir        string
}

type Engine struct {
	countdownStep  time.Duration
	countdownTicks int
	ctx            dbos.DBOSContext
	drafter        PRDrafter
	fanoutJobs     int
	greeter        Greeter
	log            *slog.Logger
	stageStep      time.Duration
	updater        Updater
	webhookTimeout time.Duration
}

func New(deps Deps) *Engine {
	if deps.CountdownStep == 0 {
		deps.CountdownStep = defaultCountdownStep
	}
	if deps.CountdownTicks == 0 {
		deps.CountdownTicks = defaultCountdownTicks
	}
	if deps.Drafter == nil {
		deps.Drafter = DefaultPRDrafter(deps.Workdir)
	}
	if deps.FanoutJobs == 0 {
		deps.FanoutJobs = defaultFanoutJobs
	}
	if deps.Greeter == nil {
		deps.Greeter = DefaultGreeter()
	}
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	if deps.StageStep == 0 {
		deps.StageStep = defaultStageStep
	}
	if deps.Updater == nil {
		deps.Updater = DefaultUpdater()
	}
	if deps.WebhookTimeout == 0 {
		deps.WebhookTimeout = defaultWebhookTimeout
	}
	e := &Engine{
		countdownStep:  deps.CountdownStep,
		countdownTicks: deps.CountdownTicks,
		ctx:            deps.DBOS,
		drafter:        deps.Drafter,
		fanoutJobs:     deps.FanoutJobs,
		greeter:        deps.Greeter,
		log:            deps.Log,
		stageStep:      deps.StageStep,
		updater:        deps.Updater,
		webhookTimeout: deps.WebhookTimeout,
	}
	if _, err := dbos.RegisterQueue(deps.DBOS, fanoutQueue, dbos.WithWorkerConcurrency(fanoutQueueWorkers)); err != nil {
		deps.Log.Error("register fanout queue", "err", err)
	}
	dbos.RegisterWorkflow(deps.DBOS, e.greet, dbos.WithWorkflowName("greet"))
	dbos.RegisterWorkflow(deps.DBOS, e.updateClaude, dbos.WithWorkflowName("update-claude"))
	dbos.RegisterWorkflow(deps.DBOS, e.createPR, dbos.WithWorkflowName("create-pr"))
	dbos.RegisterWorkflow(deps.DBOS, e.countdown, dbos.WithWorkflowName("countdown"))
	dbos.RegisterWorkflow(deps.DBOS, e.deploy, dbos.WithWorkflowName("deploy"))
	dbos.RegisterWorkflow(deps.DBOS, e.fanout, dbos.WithWorkflowName("fan-out"))
	dbos.RegisterWorkflow(deps.DBOS, e.job, dbos.WithWorkflowName("demo-job"))
	dbos.RegisterWorkflow(deps.DBOS, e.stream, dbos.WithWorkflowName("stream"))
	dbos.RegisterWorkflow(deps.DBOS, e.webhook, dbos.WithWorkflowName("webhook"))
	dbos.RegisterWorkflow(deps.DBOS, e.heartbeat)
	return e
}

type StepView struct {
	Answer   string
	Copy     string
	Detail   string
	Durable  bool
	Duration string
	Failed   bool
	Form     *elicit.Prompt
	ID       int
	Input    string
	Kind     string
	Pending  bool
	Status   string
	Summary  string
	Title    string
	Values   map[string]string
}

type RunView struct {
	Blocked bool
	ID      string
	Result  string
	Status  string
	Steps   []StepView
}

type stepBegin struct {
	Input string `json:"input"`
	Name  string `json:"name"`
	Title string `json:"title"`
}

func (v RunView) Running() bool {
	return v.Status == string(dbos.WorkflowStatusPending) || v.Status == string(dbos.WorkflowStatusEnqueued)
}

func (v RunView) Done() bool { return v.Status == string(dbos.WorkflowStatusSuccess) }

func act(ctx dbos.DBOSContext, name, title, input string, fn func(context.Context) (string, error)) (string, error) {
	if _, err := dbos.RunAsStep(ctx, func(context.Context) (stepBegin, error) {
		return stepBegin{Input: input, Name: name, Title: title}, nil
	}, dbos.WithStepName("begin/"+name)); err != nil {
		return "", errors.Wrap(err, "begin "+name)
	}
	return dbos.RunAsStep(ctx, fn, dbos.WithStepName(name))
}

func (e *Engine) Start(name, id string) error {
	var err error
	switch name {
	case "countdown":
		_, err = dbos.RunWorkflow(e.ctx, e.countdown, CountdownInput{}, dbos.WithWorkflowID(id))
	case "deploy":
		_, err = dbos.RunWorkflow(e.ctx, e.deploy, DeployInput{}, dbos.WithWorkflowID(id))
	case "fan-out":
		_, err = dbos.RunWorkflow(e.ctx, e.fanout, FanoutInput{}, dbos.WithWorkflowID(id))
	case "stream":
		_, err = dbos.RunWorkflow(e.ctx, e.stream, StreamInput{}, dbos.WithWorkflowID(id))
	case "webhook":
		_, err = dbos.RunWorkflow(e.ctx, e.webhook, WebhookInput{}, dbos.WithWorkflowID(id))
	case "update-claude":
		_, err = dbos.RunWorkflow(e.ctx, e.updateClaude, UpdateInput{}, dbos.WithWorkflowID(id))
	case "create-pr":
		_, err = dbos.RunWorkflow(e.ctx, e.createPR, PRInput{}, dbos.WithWorkflowID(id))
	default:
		_, err = dbos.RunWorkflow(e.ctx, e.greet, GreetInput{}, dbos.WithWorkflowID(id))
	}
	return errors.Wrapf(err, "start %s", name)
}

func (e *Engine) Status(id string) (string, error) {
	status, _, err := e.workflow(id)
	return status, err
}

func (e *Engine) Await(id string, timeout time.Duration) {
	deadline := timeout
	for waited := time.Duration(0); waited < deadline; waited += 25 * time.Millisecond {
		if run, err := e.Run(id); err == nil && (run.Blocked || !run.Running()) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
}

func (e *Engine) Run(id string) (RunView, error) {
	status, output, err := e.workflow(id)
	if err != nil {
		return RunView{}, err
	}
	steps, err := dbos.GetWorkflowSteps(e.ctx, id, dbos.WithStepsLoadOutput(true))
	if err != nil {
		return RunView{}, errors.Wrap(err, "get workflow steps")
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].StepID < steps[j].StepID })

	view := RunView{ID: id, Result: output, Status: status}
	var awaiting []elicit.Prompt
	var pending []stepBegin
	var evented, streamed bool
	titles := map[string]string{}
	inputs := map[string]string{}

	for _, s := range steps {
		if s.ChildWorkflowID != "" {
			continue
		}
		switch s.StepName {
		case "DBOS.setEvent":
			evented = true
		case "DBOS.writeStream":
			streamed = true
		}
		switch {
		case strings.HasPrefix(s.StepName, "begin/"):
			if b, err := decode[stepBegin](s.Output); err == nil {
				pending = append(pending, b)
				titles[b.Name] = b.Title
				inputs[b.Name] = b.Input
			}
		case strings.HasPrefix(s.StepName, "elicit/"):
			prompt, err := decode[elicit.Prompt](s.Output)
			if err != nil {
				continue
			}
			awaiting = append(awaiting, prompt)
		case s.StepName == "DBOS.recv":
			if len(awaiting) == 0 {
				continue
			}
			reply, _ := decode[elicit.Reply](s.Output)
			prompt := awaiting[0]
			sv := StepView{
				Answer:   summarizeReply(prompt, reply),
				Durable:  true,
				Duration: stepDuration(s),
				ID:       s.StepID,
				Kind:     KindForm,
				Status:   StepDone,
				Title:    prompt.Message,
			}
			if reply.Action == elicit.ActionAccept {
				sv.Form = &prompt
				sv.Values = replyValues(reply)
			}
			view.Steps = append(view.Steps, sv)
			awaiting = awaiting[1:]
		case strings.HasPrefix(s.StepName, "respond/"):
			pending = dropPending(pending, s.StepName)
			body, _ := decode[string](s.Output)
			sv := StepView{
				Detail:   body,
				Durable:  true,
				Duration: stepDuration(s),
				ID:       s.StepID,
				Input:    inputs[s.StepName],
				Kind:     KindResponse,
				Status:   stepStatus(s),
				Title:    stepDisplayTitle(titles, s.StepName),
			}
			if s.StepName == "respond/webhook" {
				sv.Copy = webhookCurl(id)
			}
			view.Steps = append(view.Steps, sv)
		case strings.HasPrefix(s.StepName, "DBOS."):
			continue
		default:
			pending = dropPending(pending, s.StepName)
			detail, _ := decode[string](s.Output)
			view.Steps = append(view.Steps, StepView{
				Detail:   detail,
				Durable:  true,
				Duration: stepDuration(s),
				Failed:   s.Error != nil,
				ID:       s.StepID,
				Input:    inputs[s.StepName],
				Kind:     KindTool,
				Status:   stepStatus(s),
				Summary:  firstLine(detail),
				Title:    stepDisplayTitle(titles, s.StepName),
			})
		}
	}

	if view.Running() && len(awaiting) > 0 {
		prompt := awaiting[len(awaiting)-1]
		view.Blocked = true
		view.Steps = append(view.Steps, StepView{
			Form:    &prompt,
			Kind:    KindForm,
			Pending: true,
			Status:  StepRunning,
			Title:   prompt.Message,
		})
	} else if view.Running() {
		for _, b := range pending {
			view.Steps = append(view.Steps, StepView{
				Input:  b.Input,
				Kind:   beginKind(b.Name),
				Status: StepRunning,
				Title:  b.Title,
			})
		}
	}
	if streamed {
		if step, ok := e.streamStep(id, view.Running()); ok {
			view.Steps = append(view.Steps, step)
		}
	}
	if evented {
		view.Steps = append(view.Steps, e.eventSteps(id, view.Running())...)
	}
	return view, nil
}

func (e *Engine) streamStep(id string, running bool) (StepView, bool) {
	lines, closed, err := dbos.ReadStream[string](e.ctx, id, streamKey, dbos.WithReadStreamSnapshot(0))
	if err != nil || len(lines) == 0 {
		return StepView{}, false
	}
	step := StepView{
		Detail:  strings.Join(lines, "\n"),
		Kind:    KindTool,
		Status:  StepRunning,
		Summary: lines[len(lines)-1],
		Title:   "Log stream",
	}
	if closed || !running {
		step.Status = StepDone
	}
	return step, true
}

func (e *Engine) eventSteps(id string, running bool) []StepView {
	version, _ := dbos.GetEvent[string](e.ctx, id, eventVersionKey, 100*time.Millisecond)
	if version == "" {
		return nil
	}
	steps := []StepView{{
		Detail: "Published with SetEvent as the deploy starts, so any caller can read it with GetEvent before the workflow finishes.",
		Kind:   KindTool,
		Status: StepDone,
		Title:  "Release " + version + " — available immediately",
	}}
	stage, _ := dbos.GetEvent[string](e.ctx, id, eventStatusKey, 100*time.Millisecond)
	status := StepRunning
	if !running {
		status = StepDone
	}
	steps = append(steps, StepView{
		Detail: stage,
		Kind:   KindTool,
		Status: status,
		Title:  "Live status",
	})
	return steps
}

func stepDuration(s dbos.StepInfo) string {
	if s.StartedAt.IsZero() || s.CompletedAt.IsZero() {
		return ""
	}
	d := s.CompletedAt.Sub(s.StartedAt)
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}

func dropPending(pending []stepBegin, name string) []stepBegin {
	out := pending[:0]
	for _, b := range pending {
		if b.Name != name {
			out = append(out, b)
		}
	}
	return out
}

func stepDisplayTitle(titles map[string]string, name string) string {
	if t, ok := titles[name]; ok && t != "" {
		return t
	}
	return stepTitle(name)
}

func beginKind(name string) string {
	if strings.HasPrefix(name, "respond/") {
		return KindResponse
	}
	return KindTool
}

func replyValues(reply elicit.Reply) map[string]string {
	if reply.Action != elicit.ActionAccept {
		return nil
	}
	var values map[string]any
	if json.Unmarshal([]byte(reply.Content), &values) != nil {
		return nil
	}
	out := map[string]string{}
	for name, value := range values {
		if value == nil {
			continue
		}
		out[name] = fmt.Sprintf("%v", value)
	}
	return out
}

func stepStatus(s dbos.StepInfo) string {
	if s.Error != nil {
		return StepFailed
	}
	return StepDone
}

func stepTitle(name string) string {
	if i := strings.IndexByte(name, '/'); i >= 0 {
		name = name[i+1:]
	}
	name = strings.ReplaceAll(name, "_", " ")
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	if len(line) > 80 {
		return line[:80] + "…"
	}
	return line
}

func (e *Engine) Resolve(id, elicitationID, action string, values map[string]string) error {
	steps, err := dbos.GetWorkflowSteps(e.ctx, id, dbos.WithStepsLoadOutput(true))
	if err != nil {
		return errors.Wrap(err, "get workflow steps")
	}
	sort.SliceStable(steps, func(i, j int) bool { return steps[i].StepID < steps[j].StepID })

	var prompt *elicit.Prompt
	elicitCount, recvCount := 0, 0
	for _, s := range steps {
		switch {
		case strings.HasPrefix(s.StepName, "elicit/"):
			elicitCount++
			if p, err := decode[elicit.Prompt](s.Output); err == nil && p.ElicitationID == elicitationID {
				prompt = &p
			}
		case s.StepName == "DBOS.recv":
			recvCount++
		}
	}
	if prompt == nil {
		return ErrFormNotFound
	}
	if recvCount >= elicitCount {
		return ErrFormResolved
	}

	reply := elicit.Reply{Action: action}
	switch action {
	case elicit.ActionAccept:
		content, err := elicit.Coerce(prompt.RequestedSchema, values)
		if err != nil {
			return err
		}
		data, err := json.Marshal(content)
		if err != nil {
			return errors.Wrap(err, "marshal content")
		}
		reply.Content = string(data)
	case elicit.ActionCancel, elicit.ActionDecline:
	default:
		return errors.Wrapf(elicit.ErrInvalid, "unknown action %q", action)
	}
	if err := dbos.Send(e.ctx, id, reply, prompt.Topic); err != nil {
		return errors.Wrap(err, "send reply")
	}
	if !e.awaitConsumed(id, elicitCount, 2*time.Second) && e.staleApplicationVersion(id) {
		return ErrFormStale
	}
	return nil
}

func (e *Engine) Fork(id, elicitationID string) (string, error) {
	steps, err := dbos.GetWorkflowSteps(e.ctx, id, dbos.WithStepsLoadOutput(true))
	if err != nil {
		return "", errors.Wrap(err, "get workflow steps")
	}
	startStep := -1
	for _, s := range steps {
		if !strings.HasPrefix(s.StepName, "elicit/") {
			continue
		}
		if p, err := decode[elicit.Prompt](s.Output); err == nil && p.ElicitationID == elicitationID {
			startStep = s.StepID
			break
		}
	}
	if startStep < 0 {
		return "", ErrFormNotFound
	}
	handle, err := dbos.ForkWorkflow[string](e.ctx, dbos.ForkWorkflowInput{
		ApplicationVersion: e.ctx.GetApplicationVersion(),
		OriginalWorkflowID: id,
		StartStep:          uint(startStep),
	})
	if err != nil {
		return "", errors.Wrap(err, "fork workflow")
	}
	return handle.GetWorkflowID(), nil
}

func (e *Engine) Cancel(id string) error {
	status, _, err := e.workflow(id)
	if errors.Is(err, ErrRunNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if status != string(dbos.WorkflowStatusPending) && status != string(dbos.WorkflowStatusEnqueued) {
		return nil
	}
	return errors.Wrap(dbos.CancelWorkflow(e.ctx, id), "cancel workflow")
}

func (e *Engine) EditForm(id, elicitationID, action string, values map[string]string) (string, error) {
	forkedID, err := e.Fork(id, elicitationID)
	if err != nil {
		return "", err
	}
	key := strings.TrimPrefix(elicitationID, id+"/")
	e.Await(forkedID, 2*time.Second)
	if err := e.Resolve(forkedID, forkedID+"/"+key, action, values); err != nil {
		return "", err
	}
	return forkedID, nil
}

func (e *Engine) awaitConsumed(id string, elicitCount int, timeout time.Duration) bool {
	for waited := time.Duration(0); waited < timeout; waited += 25 * time.Millisecond {
		steps, err := dbos.GetWorkflowSteps(e.ctx, id)
		if err == nil {
			recv := 0
			for _, s := range steps {
				if s.StepName == "DBOS.recv" {
					recv++
				}
			}
			if recv >= elicitCount {
				return true
			}
		}
		if status, _, err := e.workflow(id); err == nil && status != string(dbos.WorkflowStatusPending) && status != string(dbos.WorkflowStatusEnqueued) {
			return true
		}
		time.Sleep(25 * time.Millisecond)
	}
	return false
}

func (e *Engine) staleApplicationVersion(id string) bool {
	runs, err := dbos.ListWorkflows(e.ctx, dbos.WithWorkflowIDs([]string{id}))
	if err != nil || len(runs) == 0 {
		return false
	}
	return runs[0].ApplicationVersion != "" && runs[0].ApplicationVersion != e.ctx.GetApplicationVersion()
}

func (e *Engine) workflow(id string) (status, output string, err error) {
	runs, err := dbos.ListWorkflows(e.ctx, dbos.WithWorkflowIDs([]string{id}))
	if err != nil {
		return "", "", errors.Wrap(err, "list workflows")
	}
	if len(runs) == 0 {
		return "", "", ErrRunNotFound
	}
	run := runs[0]
	if run.Output != nil {
		output, _ = decode[string](run.Output)
	}
	return string(run.Status), output, nil
}

func decode[T any](v any) (T, error) {
	var out T
	var raw []byte
	switch t := v.(type) {
	case string:
		raw = []byte(t)
	case []byte:
		raw = t
	case json.RawMessage:
		raw = t
	default:
		data, err := json.Marshal(v)
		if err != nil {
			return out, err
		}
		raw = data
	}
	return out, json.Unmarshal(raw, &out)
}

func summarizeReply(prompt elicit.Prompt, reply elicit.Reply) string {
	switch reply.Action {
	case elicit.ActionAccept:
		var values map[string]any
		if err := json.Unmarshal([]byte(reply.Content), &values); err != nil {
			return ""
		}
		parts := make([]string, 0, len(values))
		for _, name := range prompt.Order {
			if v, ok := values[name]; ok && v != nil && v != "" {
				parts = append(parts, fmt.Sprintf("%s: %v", name, v))
			}
		}
		return strings.Join(parts, "\n")
	case elicit.ActionDecline:
		return "Declined"
	default:
		return "Dismissed"
	}
}
