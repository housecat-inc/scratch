package inbox

import (
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatBrowser(t *testing.T) {
	t.Run("inspector", func(t *testing.T) {
		t.Run("starts a chat from a captured inbox section", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.WaitChatReady()
			h.CaptureElement(".mail-mainbar")
			h.TextEventuallyContains("#chat-attachments", "selection.png")
			h.TextEventuallyContains("#chat-attachments", "selection.html")
			h.Click("#chat-send")

			h.ChatThreadCount(1)
			h.TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.png")
			h.TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.html")
			h.TextEventuallyContains("#chat-messages .role-user .chat-bubble", "Selected")
		})

		t.Run("starts a chat from a task page without creating a task", func(t *testing.T) {
			h := newHarness(t)
			h.SeedTask("broken task")
			h.Open("/inbox/tasks/1")
			h.WaitChatReady()
			h.CaptureElement(".mail-reader-title")
			h.TextEventuallyContains("#chat-attachments", "selection.png")
			h.TextEventuallyContains("#chat-attachments", "selection.html")
			h.Click("#chat-send")

			h.ChatThreadCount(1)
			h.TaskCount(1)
			h.TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.png")
			h.TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.html")
			h.TextEventuallyContains("#chat-messages .role-user .chat-bubble", "Selected")
		})

		t.Run("captures a message as image and html attachments", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.SeedAssistantMessage("Capture me")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.CaptureElement("#message-1 .chat-md")

			h.TextEventuallyContains("#chat-attachments", "selection.png")
			h.TextEventuallyContains("#chat-attachments", "selection.html")
			h.InputValueContains("#chat-input", "Selected #message-1")
			h.InputValueContains("#chat-input", "/inbox/chats/1")
		})
	})

	t.Run("composer", func(t *testing.T) {
		t.Run("sends a message and streams the echoed reply", func(t *testing.T) {
			a := assert.New(t)
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.Type("#chat-input", "hello e2e")
			h.Click("#chat-send")

			h.TextEventuallyContains("#chat-messages .role-user .chat-bubble", "hello e2e")
			h.TextEventuallyContains("#chat-messages .role-assistant", "You said: hello e2e")
			a.Equal("", h.Page.MustElement("#chat-input").MustProperty("value").Str())
		})

		t.Run("restores an unsent draft after navigation", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.Type("#chat-input", "unfinished draft")
			h.Load("/inbox/chats")
			h.Load("/inbox/chats/1")

			h.InputValueContains("#chat-input", "unfinished draft")
		})

		t.Run("sends an uploaded attachment as a pill", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.AttachFile("notes.txt", "remember this")
			h.ElementTextContains("#chat-attachments .chat-chip-name", "notes.txt")
			h.Type("#chat-input", "about the attached file")
			h.Click("#chat-send")

			h.TextEventuallyContains("#chat-messages .role-user .chat-bubble", "about the attached file")
			h.TextEventuallyContains("#chat-messages .chat-bubble-attachments .chat-chip-name", "notes.txt")
			h.TextEventuallyContains("#chat-messages .role-assistant", "You said:")
			h.ElementAbsent("#chat-attachments .chat-attachment-chip")
		})

		t.Run("toggles between full access and safe mode", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.ElementAttributeEquals(".chat-access-switch", "aria-checked", "true")
			h.Click(".chat-access-switch")

			h.ElementAttributeEquals(".chat-access-switch", "aria-checked", "false")
			thread, err := h.Chat.Thread(1)
			r.NoError(err)
			a.Equal(chat.AccessSafe, h.Chat.ThreadAccess(thread))
		})

		t.Run("stops a streaming turn", func(t *testing.T) {
			h := newHarness(t)
			h.AllowConsole("[[object Event]]")
			h.SeedChatThread("e2e chat")
			h.SeedStreamingAssistant("Working")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.Click(`[aria-label="Stop chat"]`)

			h.TextEventuallyContains("#chat-messages", "The turn was stopped.")
			h.ElementEventuallyPresent(`[aria-label="Stop chat"]`)
		})
	})

	t.Run("chat panel", func(t *testing.T) {
		t.Run("continues an existing thread", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.Click(`[data-chat-rail-thread="1"]`)
			h.PathEventuallyEquals("/inbox/chats")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
			h.Type("#chat-rail-panel [data-chat-input]", "from the panel")
			h.Click("#chat-rail-panel [data-chat-send]")

			h.TextEventuallyContains("#chat-rail-panel .role-user .chat-bubble", "from the panel")
			h.TextEventuallyContains("#chat-rail-panel .role-assistant", "You said: from the panel")
		})

		t.Run("keeps its header actions inside the panel with a long thread", func(t *testing.T) {
			r := require.New(t)
			h := newHarness(t)
			h.SeedChatThread(strings.Repeat("long thread title ", 20))
			h.SeedAssistantMessage(strings.Repeat("long thread content ", 120))
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.Click(`[data-chat-rail-thread="1"]`)
			h.ElementEventuallyPresent("#chat-rail-panel")
			r.Eventually(func() bool {
				return h.EvalBool(`() => {
					const panel = document.querySelector("#chat-rail-panel");
					const close = document.querySelector("#chat-rail-panel [data-chat-rail-close]");
					const expand = document.querySelector("#chat-rail-panel [data-chat-rail-expand]");
					if (!panel || !close || !expand) return false;
					const panelRect = panel.getBoundingClientRect();
					const inside = (el) => {
						const rect = el.getBoundingClientRect();
						return rect.width > 0 && rect.left >= panelRect.left && rect.right <= panelRect.right && rect.top >= panelRect.top;
					};
					return inside(close) && inside(expand);
				}`)
			}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)

			h.ElementVisible("#chat-rail-panel [data-chat-input]")
		})
	})

	t.Run("transcript", func(t *testing.T) {
		t.Run("renders thinking, tool, plan, and markdown parts", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.SeedAssistantMessage("",
				[2]string{chat.EventThinking, `{"text":"pondering the layout"}`},
				[2]string{chat.EventToolCall, `{"toolCallId":"tc-1","title":"Read main.go","status":"in_progress"}`},
				[2]string{chat.EventToolCallUpdate, `{"toolCallId":"tc-1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"package main"}}]}`},
				[2]string{chat.EventPlan, `{"entries":[{"content":"ship it","status":"in_progress"}]}`},
				[2]string{chat.EventDelta, `{"text":"Done with **bold** work"}`},
			)
			h.Open("/inbox/chats/1")
			h.WaitChatReady()
			h.ElementEventuallyPresent(".chat-row.row-tool summary")
			h.Click(".chat-row.row-tool summary")

			h.TextEventuallyContains(".chat-row.row-thinking summary", "Thinking")
			h.TextEventuallyContains(".chat-row.row-tool .chat-row-label", "Read main.go")
			h.TextEventuallyContains(".chat-row.row-tool .chat-row-detail", "package main")
			h.TextEventuallyContains(".chat-plan", "ship it")
			h.TextEventuallyContains(".chat-md strong", "bold")
		})
	})
}
