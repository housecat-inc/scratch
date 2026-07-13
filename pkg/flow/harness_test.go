package flow

import (
	"context"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/housecat-inc/scratch/testkit"
)

type Harness struct {
	*testkit.T
	Drafts  *atomic.Int32
	Engine  *Engine
	greetFn func(context.Context, string) (string, error)
	id      string
}

type Step = testkit.Step[*Harness]

type fakeDrafter struct {
	drafts  *atomic.Int32
	pr      PullRequest
	summary string
}

func (f fakeDrafter) Context(context.Context) (string, error) { return f.summary, nil }

func (f fakeDrafter) Draft(context.Context, string) (PullRequest, error) {
	if f.drafts != nil {
		f.drafts.Add(1)
	}
	return f.pr, nil
}

func runCases(t *testing.T, cases []testkit.Case[*Harness]) {
	testkit.RunCases(t, cases, testkit.CaseRunner[*Harness]{
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.Case[*Harness]) *Harness {
			wf, err := workflow.New(t.TempDir() + "/flow.db")
			kit.R.NoError(err)
			h := &Harness{
				T:       kit,
				Drafts:  &atomic.Int32{},
				greetFn: func(_ context.Context, name string) (string, error) { return "Hi " + name, nil },
			}
			h.Engine = New(Deps{
				DBOS:    wf.Ctx(),
				Drafter: fakeDrafter{drafts: h.Drafts, pr: PullRequest{Body: "- Add thing", Title: "Add thing"}, summary: "Commits:\nabc Add thing"},
				Greeter: GreeterFunc(func(ctx context.Context, name string) (string, error) { return h.greetFn(ctx, name) }),
				Log:     slog.Default(),
				Updater: UpdaterFuncs{
					UpdateFn:  func(context.Context) (string, error) { return "Updated to claude 2.0", nil },
					VersionFn: func(context.Context) (string, error) { return "claude 1.0", nil },
				},
			})
			kit.R.NoError(wf.Launch())
			t.Cleanup(func() { wf.Close() })
			return h
		},
	})
}

func (h *Harness) pendingForm() *elicit.Prompt {
	var prompt *elicit.Prompt
	h.R.Eventually(func() bool {
		run, err := h.Engine.Run(h.id)
		if err != nil || !run.Blocked {
			return false
		}
		for _, s := range run.Steps {
			if s.Pending && s.Form != nil {
				prompt = s.Form
				return true
			}
		}
		return false
	}, 5*time.Second, 20*time.Millisecond)
	return prompt
}

func (h *Harness) doneRun() RunView {
	var run RunView
	h.R.Eventually(func() bool {
		var err error
		run, err = h.Engine.Run(h.id)
		return err == nil && run.Done()
	}, 5*time.Second, 20*time.Millisecond)
	return run
}

func Start(name, id string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.id = id
		h.R.NoError(h.Engine.Start(name, id))
		h.Engine.Await(id, 5*time.Second)
	}
}

func ExpectForm(message string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		form := h.pendingForm()
		h.R.NotNil(form)
		h.Equal(message, form.Message)
	}
}

func Accept(values map[string]string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		form := h.pendingForm()
		h.R.NotNil(form)
		h.R.NoError(h.Engine.Resolve(h.id, form.ElicitationID, elicit.ActionAccept, values))
	}
}

func Decline() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		form := h.pendingForm()
		h.R.NotNil(form)
		h.R.NoError(h.Engine.Resolve(h.id, form.ElicitationID, elicit.ActionDecline, nil))
	}
}

func Edit(key string, values map[string]string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		forkedID, err := h.Engine.EditForm(h.id, h.id+"/"+key, elicit.ActionAccept, values)
		h.R.NoError(err)
		h.id = forkedID
	}
}

func ExpectResult(want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.Equal(want, h.doneRun().Result)
	}
}

func ExpectRunResult(id, want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		run, err := h.Engine.Run(id)
		h.R.NoError(err)
		h.Equal(want, run.Result)
	}
}

func ExpectDrafts(n int32) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.Equal(n, h.Drafts.Load())
	}
}

func ExpectStepCount(n int) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Len(h.doneRun().Steps, n)
	}
}

func ExpectStep(index int, check func(*Harness, StepView)) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		steps := h.doneRun().Steps
		h.R.Greater(len(steps), index)
		check(h, steps[index])
	}
}

func ExpectLastStep(check func(*Harness, StepView)) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		steps := h.doneRun().Steps
		h.R.NotEmpty(steps)
		check(h, steps[len(steps)-1])
	}
}

