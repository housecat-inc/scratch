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

type fakeDrafter struct {
	pr      PullRequest
	summary string
}

func (f fakeDrafter) Context(context.Context) (string, error) { return f.summary, nil }

func (f fakeDrafter) Draft(context.Context, string) (PullRequest, error) { return f.pr, nil }

func TestUpdateClaudeWorkflow(t *testing.T) {
	r := require.New(t)
	wf, err := workflow.New(t.TempDir() + "/flow.db")
	r.NoError(err)
	e := New(Deps{
		DBOS: wf.Ctx(),
		Log:  slog.Default(),
		Updater: UpdaterFuncs{
			VersionFn: func(context.Context) (string, error) { return "claude 1.0", nil },
			UpdateFn:  func(context.Context) (string, error) { return "Updated to claude 2.0", nil },
		},
	})
	r.NoError(wf.Launch())
	t.Cleanup(func() { wf.Close() })

	r.NoError(e.Start("update-claude", "u1"))
	form := waitForm(t, e, "u1")
	r.Equal("Update Claude Code to the latest version?", form.Message)
	r.NoError(e.Resolve("u1", form.ElicitationID, elicit.ActionAccept, nil))

	var run RunView
	r.Eventually(func() bool {
		run, err = e.Run("u1")
		return err == nil && run.Done()
	}, 5*time.Second, 20*time.Millisecond)
	r.Equal("Updated to claude 2.0", run.Result)
	r.Equal("Check installed version", run.Steps[0].Title)
	r.Equal(KindResponse, run.Steps[len(run.Steps)-1].Kind)
}

func TestUpdateClaudeDecline(t *testing.T) {
	r := require.New(t)
	wf, err := workflow.New(t.TempDir() + "/flow.db")
	r.NoError(err)
	e := New(Deps{
		DBOS: wf.Ctx(),
		Log:  slog.Default(),
		Updater: UpdaterFuncs{
			VersionFn: func(context.Context) (string, error) { return "claude 1.0", nil },
			UpdateFn:  func(context.Context) (string, error) { return "should not run", nil },
		},
	})
	r.NoError(wf.Launch())
	t.Cleanup(func() { wf.Close() })

	r.NoError(e.Start("update-claude", "u2"))
	form := waitForm(t, e, "u2")
	r.NoError(e.Resolve("u2", form.ElicitationID, elicit.ActionDecline, nil))

	var run RunView
	r.Eventually(func() bool {
		run, err = e.Run("u2")
		return err == nil && run.Done()
	}, 5*time.Second, 20*time.Millisecond)
	r.Empty(run.Result)
}

func TestCreatePRWorkflow(t *testing.T) {
	r := require.New(t)
	wf, err := workflow.New(t.TempDir() + "/flow.db")
	r.NoError(err)
	e := New(Deps{
		DBOS:    wf.Ctx(),
		Drafter: fakeDrafter{pr: PullRequest{Body: "- Add thing", Title: "Add thing"}, summary: "Commits:\nabc Add thing"},
		Log:     slog.Default(),
	})
	r.NoError(wf.Launch())
	t.Cleanup(func() { wf.Close() })

	r.NoError(e.Start("create-pr", "pr1"))
	form := waitForm(t, e, "pr1")
	r.Equal("Review the pull request", form.Message)

	r.NoError(e.Resolve("pr1", form.ElicitationID, elicit.ActionAccept, map[string]string{
		"body":  "- Add thing",
		"title": "Add thing",
	}))

	var run RunView
	r.Eventually(func() bool {
		run, err = e.Run("pr1")
		return err == nil && run.Done()
	}, 5*time.Second, 20*time.Millisecond)
	r.Equal("Add thing", run.Result)
	r.Equal("Collect changes", run.Steps[0].Title)
	r.Equal("Draft release notes", run.Steps[1].Title)
	r.Contains(run.Steps[len(run.Steps)-1].Detail, "Add thing")
}
