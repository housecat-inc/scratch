package flow

import (
	"log/slog"
	"path/filepath"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/require"
)

func TestGreetWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "accept prompts for a name then greets",
			Act: []Step{
				Start("greet", "greet-accept"),
				ExpectForm("What should I call you?"),
				Accept(map[string]string{"name": "Ada"}),
			},
			Assert: []Step{
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
			Name: "decline ends the workflow",
			Act: []Step{
				Start("greet", "greet-decline"),
				ExpectForm("What should I call you?"),
				Decline(),
			},
			Assert: []Step{
				ExpectResult(""),
				ExpectStepCount(1),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal(KindForm, s.Kind)
					h.Equal("Declined", s.Answer)
				}),
			},
		},
		{
			Name: "edit reruns from the name form with a new value",
			Act: []Step{
				Start("greet", "greet-edit"),
				Accept(map[string]string{"name": "Ada"}),
				ExpectResult("Hi Ada"),
				Edit("name", map[string]string{"name": "Grace"}),
			},
			Assert: []Step{
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
	})
}

func TestUpdateClaudeWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "accept runs the update",
			Act: []Step{
				Start("update-claude", "update-accept"),
				ExpectForm("Update Claude Code to the latest version?"),
				Accept(nil),
			},
			Assert: []Step{
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
			Name: "decline skips the update",
			Act: []Step{
				Start("update-claude", "update-decline"),
				ExpectForm("Update Claude Code to the latest version?"),
				Decline(),
			},
			Assert: []Step{ExpectResult("")},
		},
	})
}

func TestCreatePRWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "accept drafts and finalizes the pull request",
			Act: []Step{
				Start("create-pr", "pr-accept"),
				ExpectForm("Review the pull request"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
			},
			Assert: []Step{
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
			Name: "edit the review reuses the cached draft",
			Act: []Step{
				Start("create-pr", "pr-edit"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
				ExpectResult("Add thing"),
				ExpectDrafts(1),
				Edit("review", map[string]string{"body": "- Add a better thing", "title": "Add a better thing"}),
			},
			Assert: []Step{
				ExpectResult("Add a better thing"),
				ExpectDrafts(1),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.R.Contains(s.Detail, "Add a better thing")
				}),
			},
		},
		{
			Name: "fork the review shows a fresh form with the cached draft",
			Act: []Step{
				Start("create-pr", "pr-fork"),
				Accept(map[string]string{"body": "- Add thing", "title": "Add thing"}),
				ExpectResult("Add thing"),
				ExpectDrafts(1),
				Fork("review"),
			},
			Assert: []Step{
				ExpectForm("Review the pull request"),
				ExpectDrafts(1),
			},
		},
	})
}

func TestForkUsesCurrentApplicationVersion(t *testing.T) {
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
}

func TestNormalizePullRequestUnescapesHTMLEntities(t *testing.T) {
	r := require.New(t)

	pr := normalizePullRequest(PullRequest{
		Body:  "- Add edit &amp; fork actions",
		Title: "Add edit &amp; fork actions",
	})

	r.Equal("- Add edit & fork actions", pr.Body)
	r.Equal("Add edit & fork actions", pr.Title)
}
