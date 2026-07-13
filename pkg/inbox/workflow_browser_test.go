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
				TextContains(".chat-turn.role-user .chat-bubble", "Ada"),
				TextContains(".chat-turn.role-assistant .chat-md", "Ada"),
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
				TextContains(".chat-turn.role-user .chat-bubble", "Declined"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "declining the name form ends the workflow",
			Path: "/",
		},
	})
}
