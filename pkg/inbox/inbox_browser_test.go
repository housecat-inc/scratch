package inbox

import "testing"

func TestInboxBrowser(t *testing.T) {
	t.Run("new chat", func(t *testing.T) {
		tests := []struct {
			model string
			name  string
		}{
			{model: "codex:default", name: "opens the chat panel when the default provider is selected"},
			{model: "echo:default", name: "opens the chat panel when a provider is selected"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.Open("/")
				h.SelectOption(`[name="provider_model"]`, tt.model)

				h.ChatThreadCount(1)
				h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
				h.ElementTextContains("#chat-rail-panel", "New chat")
			})
		}

		t.Run("opens the chat panel from the new chat action", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-new-chat]")

			h.ChatThreadCount(1)
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
			h.ElementTextContains("#chat-rail-panel", "New chat")
		})
	})

	t.Run("composer", func(t *testing.T) {
		tests := []struct {
			name string
			path string
			text string
		}{
			{name: "creates the thread when sending from a draft chat", path: "/inbox/chats/new", text: "hello from draft"},
			{name: "starts a chat from the tasks footer", path: "/inbox/tasks", text: "buy milk"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				h := newHarness(t)
				h.Open(tt.path)
				h.Type("#chat-input", tt.text)
				h.Click("#chat-send")

				h.ChatThreadCount(1)
				h.ElementTextContains(".mail-reader-title", tt.text)
			})
		}
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
