package inbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCodexChatBrowser(t *testing.T) {
	t.Run("starts a new chat and streams the codex reply", func(t *testing.T) {
		a := assert.New(t)
		h := newCodexHarness(t)
		h.Open("/")
		h.Click("[data-chat-rail-toggle]")
		h.ElementEventuallyPresent(railPrompt)
		h.Type(railPrompt, "hello codex")
		h.Press(railPrompt, "Enter")

		h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "codex heard: hello codex")
		h.EventuallyTrue(`() => new URLSearchParams(location.search).get("chat") === "1"`)
		h.ChatThreadCount(1)
		threads, err := h.Chat.Threads()
		a.NoError(err)
		a.Equal("hello codex", threads[0].Title)
	})

	t.Run("resumes the codex thread for a follow-up", func(t *testing.T) {
		h := newCodexHarness(t)
		h.Open("/?chat=new")
		h.ElementEventuallyPresent(railPrompt)
		h.Type(railPrompt, "first")
		h.Press(railPrompt, "Enter")
		h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "codex heard: first")
		h.Type(railPrompt, "second")
		h.Press(railPrompt, "Enter")

		h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "codex again heard: second")
	})

	t.Run("lists the new chat under chats", func(t *testing.T) {
		h := newCodexHarness(t)
		h.Open("/?chat=new")
		h.ElementEventuallyPresent(railPrompt)
		h.Type(railPrompt, "list me")
		h.Press(railPrompt, "Enter")
		h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "codex heard: list me")
		h.Open("/inbox/chats")

		h.TextEventuallyContains("[data-mail-content]", "list me")
	})
}
