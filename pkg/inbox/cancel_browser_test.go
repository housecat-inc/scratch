package inbox

import "testing"

func TestCancelBrowser(t *testing.T) {
	t.Run("cancelling a durable run exposes a resume control on the row", func(t *testing.T) {
		h := newHarness(t)
		h.Open("/")
		h.SelectOption(`[name="workflow_type"]`, "webhook")
		h.ElementTextContains("#wf-body", "Waiting for an external callback")
		h.ElementTextContains(".mail-reader-head", "Running")
		h.Click(`#wf-status form[action$="/cancel"] button`)
		h.AwaitWorkflowRunning(false)
		h.Load("/inbox/workflows")
		h.Hover(".gm-row")

		h.ElementVisible(`.gm-row form[action$="/resume"] button`)
		h.ElementAbsent(`.gm-row form[action$="/cancel"] button`)
	})

	t.Run("resuming a cancelled run restarts it and restores the cancel control", func(t *testing.T) {
		h := newHarness(t)
		h.Open("/")
		h.SelectOption(`[name="workflow_type"]`, "webhook")
		h.ElementTextContains("#wf-body", "Waiting for an external callback")
		h.Click(`#wf-status form[action$="/cancel"] button`)
		h.AwaitWorkflowRunning(false)
		h.Load("/inbox/workflows")
		h.Hover(".gm-row")
		h.Click(`.gm-row form[action$="/resume"] button`)
		h.AwaitWorkflowRunning(true)
		h.Load("/inbox/workflows")
		h.Hover(".gm-row")

		h.ElementVisible(`.gm-row form[action$="/cancel"] button`)
		h.ElementAbsent(`.gm-row form[action$="/resume"] button`)
	})
}
