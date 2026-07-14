package flow

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
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
