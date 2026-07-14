package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func editableFormValue(name, want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			v, err := h.Page.Eval(`(name) => {
				const el = document.querySelector('.chat-elicit-editable [name="' + name + '"]');
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
				TextContains(".chat-row-label", "What should I call you?"),
				Type("[name=f_name]", "Ada"),
				Click("#elicit-accept"),
			},
			Assert: []Step{
				editableFormValue("f_name", "Ada"),
				TextContains(".chat-turn.role-assistant .chat-md", "Ada"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "greet workflow prompts for a name then shows a greeting",
			Path: "/",
		},
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "update-claude"),
			},
			Assert: []Step{
				TextContains(".mail-reader-title", "Update Claude Code"),
				TextContains("#wf-body", "Update Claude Code to the latest version?"),
			},
			Name: "workflow selection starts the selected workflow",
			Path: "/",
		},
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "greet"),
			},
			Assert: []Step{
				TextContains(".mail-reader-title", "Greet"),
				TextContains(".chat-row-label", "What should I call you?"),
			},
			Name: "greet workflow selection starts the greet workflow",
			Path: "/",
		},
		{
			Act: []Step{
				Click("[data-new-workflow]"),
				TextContains(".chat-row-label", "What should I call you?"),
				Click("#elicit-decline"),
			},
			Assert: []Step{
				TextContains(".chat-turn.role-user .chat-bubble", "Declined"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "declining the name form ends the workflow",
			Path: "/",
		},
		{
			Act: []Step{
				Click("[data-new-workflow]"),
				Type("[name=f_name]", "Ada"),
				Click("#elicit-accept"),
				TextContains(".chat-turn.role-assistant .chat-md", "Ada"),
				Hover(".chat-elicit-editable"),
				Click("[data-elicit-edit]"),
				Visible("[data-elicit-submit]"),
				Visible("[data-elicit-cancel]"),
				Fill(".chat-elicit-editable [name=f_name]", "Grace"),
				Click("[data-elicit-submit]"),
			},
			Assert: []Step{
				editableFormValue("f_name", "Grace"),
				TextContains(".chat-turn.role-assistant .chat-md", "Grace"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "editing the name reruns the greeting with a new value",
			Path: "/",
		},
		{
			Act: []Step{
				Click("[data-new-workflow]"),
				Type("[name=f_name]", "Ada"),
				Click("#elicit-accept"),
				TextContains(".chat-turn.role-assistant .chat-md", "Ada"),
				Hover(".chat-elicit-editable"),
				Click("[data-elicit-fork]"),
				TextContains(".chat-row-label", "What should I call you?"),
				Type("[name=f_name]", "Bob"),
				Click("#elicit-accept"),
			},
			Assert: []Step{
				TextContains(".chat-turn.role-assistant .chat-md", "Bob"),
				TextContains(".mail-reader-head", "Completed"),
			},
			Name: "forking the name form branches into a new runnable workflow",
			Path: "/",
		},
	})
}
