package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func filledFormValue(name, want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			v, err := h.Page.Eval(`(name) => {
				const el = document.querySelector('.chat-elicit-filled [name="' + name + '"]');
				return el ? (el.value + '|' + el.disabled) : "";
			}`, name)
			return err == nil && v.Value.Str() == want+"|true"
		}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
	}
}

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
				filledFormValue("f_name", "Ada"),
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
