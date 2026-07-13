package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestWorkflowGreetBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				Click("[data-new-workflow]"),
				TextContains(".chat-elicit-message", "What should I call you?"),
				Type("[name=f_name]", "Ada"),
				Click("#elicit-accept"),
			},
			Assert: []Step{
				TextContains(".wf-steps", "Generate greeting"),
				TextContains(".wf-steps", "Ada"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "greet workflow prompts for a name then shows a greeting",
			Path: "/",
		},
		{
			Act: []Step{
				Click("[data-new-workflow]"),
				TextContains(".chat-elicit-message", "What should I call you?"),
				Click("#elicit-decline"),
			},
			Assert: []Step{
				TextContains(".wf-steps", "Declined"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "declining the name form ends the workflow",
			Path: "/",
		},
	})
}
