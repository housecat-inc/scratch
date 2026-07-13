package flow

import (
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
)

var (
	ErrFormNotFound = errors.New("form not found")
	ErrFormResolved = errors.New("form already resolved")
	ErrRunNotFound  = errors.New("workflow run not found")
)

type Deps struct {
	DBOS    dbos.DBOSContext
	Greeter Greeter
	Log     *slog.Logger
}

type Engine struct {
	ctx     dbos.DBOSContext
	greeter Greeter
	log     *slog.Logger
}

func New(deps Deps) *Engine {
	if deps.Greeter == nil {
		deps.Greeter = DefaultGreeter()
	}
	if deps.Log == nil {
		deps.Log = slog.Default()
	}
	e := &Engine{ctx: deps.DBOS, greeter: deps.Greeter, log: deps.Log}
	dbos.RegisterWorkflow(deps.DBOS, e.greet, dbos.WithWorkflowName("greet"))
	return e
}

type StepView struct {
	Detail string
	Status string
	Title  string
}

type RunView struct {
	Form   *elicit.Prompt
	ID     string
	Result string
	Status string
	Steps  []StepView
}

func (v RunView) Running() bool {
	return v.Status == string(dbos.WorkflowStatusPending) || v.Status == string(dbos.WorkflowStatusEnqueued)
}

func (v RunView) Done() bool { return v.Status == string(dbos.WorkflowStatusSuccess) }

func (e *Engine) Start(id string) error {
	_, err := dbos.RunWorkflow(e.ctx, e.greet, GreetInput{}, dbos.WithWorkflowID(id))
	return errors.Wrap(err, "start greet")
}

func (e *Engine) Status(id string) (string, error) {
	status, _, err := e.workflow(id)
	return status, err
}

func (e *Engine) Await(id string, timeout time.Duration) {
	deadline := timeout
	for waited := time.Duration(0); waited < deadline; waited += 25 * time.Millisecond {
		if run, err := e.Run(id); err == nil && (run.Form != nil || !run.Running()) {
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
	type pending struct {
		prompt elicit.Prompt
		step   *StepView
	}
	var awaiting []pending
	var rendered []*StepView

	for _, s := range steps {
		switch {
		case strings.HasPrefix(s.StepName, "elicit/"):
			prompt, err := decode[elicit.Prompt](s.Output)
			if err != nil {
				continue
			}
			sv := &StepView{Status: StepRunning, Title: prompt.Message}
			rendered = append(rendered, sv)
			awaiting = append(awaiting, pending{prompt: prompt, step: sv})
		case s.StepName == "DBOS.recv":
			if len(awaiting) == 0 {
				continue
			}
			reply, _ := decode[elicit.Reply](s.Output)
			awaiting[0].step.Detail = summarizeReply(awaiting[0].prompt, reply)
			awaiting[0].step.Status = StepDone
			awaiting = awaiting[1:]
		case s.StepName == "greeting":
			greeting, _ := decode[string](s.Output)
			rendered = append(rendered, &StepView{Detail: greeting, Status: StepDone, Title: "Generate greeting"})
		}
	}

	if view.Running() && len(awaiting) > 0 {
		last := awaiting[len(awaiting)-1]
		prompt := last.prompt
		view.Form = &prompt
		last.step.Status = StepRunning
	}
	for _, s := range rendered {
		view.Steps = append(view.Steps, *s)
	}
	return view, nil
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
	e.awaitConsumed(id, elicitCount, 2*time.Second)
	return nil
}

func (e *Engine) awaitConsumed(id string, elicitCount int, timeout time.Duration) {
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
				return
			}
		}
		if status, _, err := e.workflow(id); err == nil && status != string(dbos.WorkflowStatusPending) && status != string(dbos.WorkflowStatusEnqueued) {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
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
