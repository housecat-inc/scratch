package inbox

import "testing"

func TestInboxBrowser(t *testing.T) {
	t.Run("new chat", func(t *testing.T) {
		t.Run("opens the chat panel from the new chat action", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-chat]")

			h.ElementEventuallyPresent("#mail-chat-rail-content [data-prompt-field]")
			h.TextEventuallyContains("#mail-chat-rail-content", "New Chat")
		})
	})

	t.Run("tasks", func(t *testing.T) {
		t.Run("hides done and archived tasks under the active filter", func(t *testing.T) {
			h := newHarness(t)
			h.SeedTask("active one")
			h.SeedTask("done one", TaskCompleted(true))
			h.SeedTask("archived one", TaskArchived(true))
			h.Open("/inbox/tasks?archived=active")

			h.ElementTextContains(".mail-list", "active one")
			h.ElementAbsent(`[data-id="2"]`)
			h.ElementAbsent(`[data-id="3"]`)
		})

		t.Run("saves an edited description", func(t *testing.T) {
			h := newHarness(t)
			h.SeedTask("draft")
			h.Open("/inbox/tasks/1")
			h.Type(`[name="description"]`, "Call customer before Friday")
			h.Click(".mail-save-btn")

			h.TaskDescription(1, "Call customer before Friday")
			h.ElementTextContains(".mail-reader-title", "draft")
		})
	})
}
