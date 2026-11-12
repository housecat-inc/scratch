package flow

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func TestWorkflows(t *testing.T) {

	t.Run("when the application version changes", func(t *testing.T) {
		t.Run("forks a run onto the current version", func(t *testing.T) {
			r := require.New(t)
			path := filepath.Join(t.TempDir(), "flow.db")
			drafter := fakeDrafter{pr: PullRequest{Body: "- Add thing", Title: "Add thing"}, summary: "Commits:\nabc Add thing"}

			t.Setenv("DBOS__APPVERSION", "v1")
			wf1, err := workflow.New(path)
			r.NoError(err)
			e1 := New(Deps{
				DBOS:    wf1.Ctx(),
				Drafter: drafter,
				Log:     slog.Default(),
			})
			r.NoError(wf1.Launch())
			r.NoError(e1.Start("create-pr", "pr-v1"))
			e1.Await("pr-v1", 5*time.Second)
			run, err := e1.Run("pr-v1")
			r.NoError(err)
			r.True(run.Blocked)
			r.NotEmpty(run.Steps)
			form := run.Steps[len(run.Steps)-1].Form
			r.NotNil(form)
			r.NoError(e1.Resolve("pr-v1", form.ElicitationID, elicit.ActionAccept, map[string]string{"body": "- Add thing", "title": "Add thing"}))
			e1.Await("pr-v1", 5*time.Second)
			r.NoError(wf1.Close())

			t.Setenv("DBOS__APPVERSION", "v2")
			wf2, err := workflow.New(path)
			r.NoError(err)
			e2 := New(Deps{
				DBOS:    wf2.Ctx(),
				Drafter: drafter,
				Log:     slog.Default(),
			})
			r.NoError(wf2.Launch())
			t.Cleanup(func() { wf2.Close() })

			forkedID, err := e2.Fork("pr-v1", "pr-v1/review")
			r.NoError(err)
			e2.Await(forkedID, 5*time.Second)
			run, err = e2.Run(forkedID)
			r.NoError(err)
			r.True(run.Blocked)
			r.NotEmpty(run.Steps)
			r.NotNil(run.Steps[len(run.Steps)-1].Form)
		})

		t.Run("rejects resolving an old form as stale but reruns it on edit", func(t *testing.T) {
			r := require.New(t)
			path := filepath.Join(t.TempDir(), "flow.db")
			drafter := fakeDrafter{pr: PullRequest{Body: "- Add thing", Title: "Add thing"}, summary: "Commits:\nabc Add thing"}

			t.Setenv("DBOS__APPVERSION", "v1")
			wf1, err := workflow.New(path)
			r.NoError(err)
			e1 := New(Deps{
				DBOS:    wf1.Ctx(),
				Drafter: drafter,
				Log:     slog.Default(),
			})
			r.NoError(wf1.Launch())
			r.NoError(e1.Start("create-pr", "pr-stale"))
			e1.Await("pr-stale", 5*time.Second)
			run, err := e1.Run("pr-stale")
			r.NoError(err)
			r.True(run.Blocked)
			r.NoError(wf1.Close())

			t.Setenv("DBOS__APPVERSION", "v2")
			wf2, err := workflow.New(path)
			r.NoError(err)
			e2 := New(Deps{
				DBOS:    wf2.Ctx(),
				Drafter: drafter,
				Log:     slog.Default(),
			})
			r.NoError(wf2.Launch())
			t.Cleanup(func() { wf2.Close() })

			err = e2.Resolve("pr-stale", "pr-stale/review", elicit.ActionAccept, map[string]string{"body": "- Add thing", "title": "Add thing"})
			r.ErrorIs(err, ErrFormStale)

			forkedID, err := e2.EditForm("pr-stale", "pr-stale/review", elicit.ActionAccept, map[string]string{"body": "- Add thing", "title": "Add thing"})
			r.NoError(err)
			e2.Await(forkedID, 5*time.Second)
			run, err = e2.Run(forkedID)
			r.NoError(err)
			r.True(run.Done())
			r.Equal("Add thing", run.Result)
		})
	})
}

func TestCreatePRWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "drafts and finalizes the pull request on accept",
			act: []Step{
				Start("create-pr", "pr-accept"),
				ExpectForm("Review the pull request"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
			},
			assert: []Step{
				ExpectResult("Add thing"),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal("Collect changes", s.Title)
				}),
				ExpectStep(1, func(h *Harness, s StepView) {
					h.Equal("Draft release notes", s.Title)
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.R.Contains(s.Detail, "Add thing")
				}),
			},
		},
		{
			name: "reuses the cached draft when the review is edited",
			act: []Step{
				Start("create-pr", "pr-edit"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
				ExpectResult("Add thing"),
				ExpectDrafts(1),
				Edit("review", map[string]string{"body": "- Add a better thing", "title": "Add a better thing"}),
			},
			assert: []Step{
				ExpectResult("Add a better thing"),
				ExpectDrafts(1),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.R.Contains(s.Detail, "Add a better thing")
				}),
			},
		},
		{
			name: "shows a fresh review form with the cached draft when forked",
			act: []Step{
				Start("create-pr", "pr-fork"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
				ExpectResult("Add thing"),
				ExpectDrafts(1),
				Fork("review"),
			},
			assert: []Step{
				ExpectForm("Review the pull request"),
				ExpectDrafts(1),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}

func TestGreetWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "prompts for a name then greets on accept",
			act: []Step{
				Start("greet", "greet-accept"),
				ExpectForm("What should I call you?"),
				Accept(map[string]string{"name": "Ada"}),
			},
			assert: []Step{
				ExpectResult("Hi Ada"),
				ExpectStepCount(2),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal(KindForm, s.Kind)
					h.Equal(StepDone, s.Status)
					h.Equal("Ada", s.Values["name"])
				}),
				ExpectStep(1, func(h *Harness, s StepView) {
					h.Equal(KindResponse, s.Kind)
					h.Equal("Generate greeting", s.Title)
					h.Equal("Hi Ada", s.Detail)
				}),
			},
		},
		{
			name: "ends without a greeting on decline",
			act: []Step{
				Start("greet", "greet-decline"),
				ExpectForm("What should I call you?"),
				Decline(),
			},
			assert: []Step{
				ExpectResult(""),
				ExpectStepCount(1),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal(KindForm, s.Kind)
					h.Equal("Declined", s.Answer)
				}),
			},
		},
		{
			name: "reruns from the edited name form and keeps the original run",
			act: []Step{
				Start("greet", "greet-edit"),
				Accept(map[string]string{"name": "Ada"}),
				ExpectResult("Hi Ada"),
				Edit("name", map[string]string{"name": "Grace"}),
			},
			assert: []Step{
				ExpectResult("Hi Grace"),
				ExpectStepCount(2),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal("Grace", s.Values["name"])
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Hi Grace", s.Detail)
				}),
				ExpectRunResult("greet-edit", "Hi Ada"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}

func TestNormalizePullRequest(t *testing.T) {
	t.Run("unescapes HTML entities", func(t *testing.T) {
		r := require.New(t)

		pr := normalizePullRequest(PullRequest{
			Body:  "- Add edit &amp; fork actions",
			Title: "Add edit &amp; fork actions",
		})

		r.Equal("- Add edit & fork actions", pr.Body)
		r.Equal("Add edit & fork actions", pr.Title)
	})
}

func TestUpdateClaudeWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "runs the update on accept",
			act: []Step{
				Start("update-claude", "update-accept"),
				ExpectForm("Update Claude Code to the latest version?"),
				Accept(nil),
			},
			assert: []Step{
				ExpectResult("Updated to claude 2.0"),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal("Check installed version", s.Title)
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal(KindResponse, s.Kind)
				}),
			},
		},
		{
			name: "skips the update on decline",
			act: []Step{
				Start("update-claude", "update-decline"),
				ExpectForm("Update Claude Code to the latest version?"),
				Decline(),
			},
			assert: []Step{ExpectResult("")},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}
