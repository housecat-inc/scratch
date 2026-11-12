package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/require"
)

func TestWorkflowBrowser(t *testing.T) {
	t.Run("greet", func(t *testing.T) {
		t.Run("prompts for a name then shows a greeting", func(t *testing.T) {
			r := require.New(t)
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-workflow]")
			h.ElementTextContains(".chat-row-label", "What should I call you?")
			h.Type("[name=f_name]", "Ada")
			h.Click("#elicit-accept")

			r.Eventually(func() bool { return editableFormValue(h, "f_name") == "Ada|true" }, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
			h.ElementTextContains(".chat-turn.role-assistant .chat-md", "Ada")
			h.ElementTextContains(".mail-reader-head", "Completed")
		})

		t.Run("starts when selected from the workflow menu", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.SelectOption(`[name="workflow_type"]`, "greet")

			h.ElementTextContains(".mail-reader-title", "Greet")
			h.ElementTextContains(".chat-row-label", "What should I call you?")
		})

		t.Run("ends when the name form is declined", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-workflow]")
			h.ElementTextContains(".chat-row-label", "What should I call you?")
			h.Click("#elicit-decline")

			h.ElementTextContains(".chat-turn.role-user .chat-bubble", "Declined")
			h.ElementTextContains(".mail-reader-head", "Completed")
		})

		t.Run("reruns the greeting after editing the name", func(t *testing.T) {
			r := require.New(t)
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-workflow]")
			h.Type("[name=f_name]", "Ada")
			h.Click("#elicit-accept")
			h.ElementTextContains(".chat-turn.role-assistant .chat-md", "Ada")
			h.Hover(".chat-elicit-editable")
			h.Click("[data-elicit-edit]")
			h.ElementVisible("[data-elicit-submit]")
			h.ElementVisible("[data-elicit-cancel]")
			h.Fill(".chat-elicit-editable [name=f_name]", "Grace")
			h.Click("[data-elicit-submit]")

			r.Eventually(func() bool { return editableFormValue(h, "f_name") == "Grace|true" }, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
			h.ElementTextContains(".chat-turn.role-assistant .chat-md", "Grace")
			h.ElementTextContains(".mail-reader-head", "Completed")
		})

		t.Run("forks the name form into a new runnable workflow", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-workflow]")
			h.Type("[name=f_name]", "Ada")
			h.Click("#elicit-accept")
			h.ElementTextContains(".chat-turn.role-assistant .chat-md", "Ada")
			h.Hover(".chat-elicit-editable")
			h.Click("[data-elicit-fork]")
			h.ElementTextContains(".chat-row-label", "What should I call you?")
			h.Type("[name=f_name]", "Bob")
			h.Click("#elicit-accept")

			h.ElementTextContains(".chat-turn.role-assistant .chat-md", "Bob")
			h.ElementTextContains(".mail-reader-head", "Completed")
		})
	})

	t.Run("stream", func(t *testing.T) {
		t.Run("refreshes the header to completed without a manual reload", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.SelectOption(`[name="workflow_type"]`, "stream")
			h.ElementTextContains(".mail-reader-head", "Running")

			h.ElementTextContains(".mail-reader-head", "Completed")
			h.ElementTextContains("#wf-body", "Done ✓")
		})
	})

	t.Run("update claude", func(t *testing.T) {
		t.Run("starts when selected from the workflow menu", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.SelectOption(`[name="workflow_type"]`, "update-claude")

			h.ElementTextContains(".mail-reader-title", "Update Claude Code")
			h.ElementTextContains("#wf-body", "Update Claude Code to the latest version?")
		})
	})
}

func editableFormValue(h *Harness, name string) string {
	v, err := h.Page.Eval(`(name) => {
		const el = document.querySelector('.chat-elicit-editable [name="' + name + '"]');
		return el ? (el.value + '|' + el.disabled) : "";
	}`, name)
	if err != nil {
		return ""
	}
	return v.Value.Str()
}
