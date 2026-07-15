package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestWorkflowCancelResumeBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "webhook"),
				TextContains("#wf-body", "Waiting for an external callback"),
				TextContains(".mail-reader-head", "Running"),
				Click(`#wf-status form[action$="/cancel"] button`),
				Load("/inbox/workflows"),
				Hover(".gm-row"),
			},
			Assert: []Step{
				Visible(`.gm-row form[action$="/resume"] button`),
				ElementAbsent(`.gm-row form[action$="/cancel"] button`),
			},
			Name: "cancelling a durable run exposes a resume control on the row",
			Path: "/",
		},
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "webhook"),
				TextContains("#wf-body", "Waiting for an external callback"),
				Click(`#wf-status form[action$="/cancel"] button`),
				Load("/inbox/workflows"),
				Hover(".gm-row"),
				Click(`.gm-row form[action$="/resume"] button`),
				Load("/inbox/workflows"),
				Hover(".gm-row"),
			},
			Assert: []Step{
				Visible(`.gm-row form[action$="/cancel"] button`),
				ElementAbsent(`.gm-row form[action$="/resume"] button`),
			},
			Name: "resuming a cancelled run restarts it and restores the cancel control",
			Path: "/",
		},
	})
}
