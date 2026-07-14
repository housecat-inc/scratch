package flow

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func newEngine(t *testing.T) *Engine {
	t.Helper()
	wf, err := workflow.New(t.TempDir() + "/flow.db")
	require.NoError(t, err)
	engine := New(Deps{
		DBOS:    wf.Ctx(),
		Greeter: GreeterFunc(func(_ context.Context, name string) (string, error) { return "Hi " + name, nil }),
		Log:     slog.Default(),
	})
	require.NoError(t, wf.Launch())
	t.Cleanup(func() { wf.Close() })
	return engine
}

func waitPrompt(t *testing.T, e *Engine, id string) *elicit.Prompt {
	t.Helper()
	var prompt *elicit.Prompt
	require.Eventually(t, func() bool {
		run, err := e.Run(id)
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

func TestGreetProgressWhileRunning(t *testing.T) {
	r := require.New(t)
	release := make(chan struct{})
	wf, err := workflow.New(t.TempDir() + "/flow.db")
	r.NoError(err)
	e := New(Deps{
		DBOS: wf.Ctx(),
		Greeter: GreeterFunc(func(_ context.Context, name string) (string, error) {
			<-release
			return "Hi " + name, nil
		}),
		Log: slog.Default(),
	})
	r.NoError(wf.Launch())
	t.Cleanup(func() { close(release); wf.Close() })

	r.NoError(e.Start("greet", "greet-p"))
	form := waitPrompt(t, e, "greet-p")
	r.NoError(e.Resolve("greet-p", form.ElicitationID, elicit.ActionAccept, map[string]string{"name": "Ada"}))

	var run RunView
	r.Eventually(func() bool {
		run, err = e.Run("greet-p")
		if err != nil || len(run.Steps) == 0 {
			return false
		}
		return run.Steps[len(run.Steps)-1].Status == StepRunning
	}, 5*time.Second, 20*time.Millisecond)
	last := run.Steps[len(run.Steps)-1]
	r.Equal("Generate greeting", last.Title)
	r.Equal(KindResponse, last.Kind)
	r.False(run.Blocked)
	release <- struct{}{}
}

func TestResolveUnknownForm(t *testing.T) {
	r := require.New(t)
	e := newEngine(t)

	r.NoError(e.Start("greet", "greet-unknown"))
	waitPrompt(t, e, "greet-unknown")

	err := e.Resolve("greet-unknown", "greet-unknown/missing", elicit.ActionAccept, map[string]string{"name": "Ada"})
	r.ErrorIs(err, ErrFormNotFound)
}
