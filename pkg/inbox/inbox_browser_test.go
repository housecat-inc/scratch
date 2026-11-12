package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
)

func TestInboxBrowser(t *testing.T) {
	t.Run("chats", func(t *testing.T) {
		t.Run("hides archived chats by default", func(t *testing.T) {
			h := newHarness(t)
			h.SeedRailThread("open chat")
			archived := h.SeedRailThread("archived chat")
			h.R.NoError(h.Chat.SetThreadState(archived, db.ThreadStateArchived))
			h.Open("/inbox/chats")

			h.ElementTextContains(".mail-list", "open chat")
			h.ElementAbsent(`[data-kind="chat"][data-id="2"]`)
			h.ElementTextContains(".mail-filter-tab.active", "Active")
		})

		t.Run("shows archived chats under the all filter", func(t *testing.T) {
			h := newHarness(t)
			h.SeedRailThread("open chat")
			archived := h.SeedRailThread("archived chat")
			h.R.NoError(h.Chat.SetThreadState(archived, db.ThreadStateArchived))
			h.Open("/inbox/chats")
			h.Click(`.mail-filter-tab[href="/inbox/chats?archived=all"]`)

			h.PathEventuallyEquals("/inbox/chats")
			h.ElementEventuallyPresent(`[data-kind="chat"][data-id="2"]`)
			h.ElementTextContains(".mail-filter-tab.active", "All")
		})

		t.Run("archives a chat from its row and drops it from the list", func(t *testing.T) {
			h := newHarness(t)
			h.SeedRailThread("to archive")
			h.Open("/inbox/chats")
			h.Hover(`[data-kind="chat"][data-id="1"]`)
			h.Click(`[data-kind="chat"][data-id="1"] [aria-label="Archive"]`)

			h.EventuallyTrue(`() => !document.querySelector('[data-kind="chat"][data-id="1"]')`)
			h.R.Eventually(func() bool {
				thread, err := h.Chat.Thread(1)
				return err == nil && thread.State == db.ThreadStateArchived
			}, 5e9, 1e8)
		})

		t.Run("opens a row in the chat rail", func(t *testing.T) {
			h := newHarness(t)
			h.SeedRailThread("from the row")
			h.Open("/inbox/chats")
			h.Click(`[data-kind="chat"][data-id="1"] .gm-row-link`)

			h.PathEventuallyEquals("/inbox/chats")
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "from the row")
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
