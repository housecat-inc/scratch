package inbox

import (
	"fmt"
	"testing"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const railPrompt = "#mail-chat-rail-content [data-prompt-field]"

func TestChatRailBrowser(t *testing.T) {
	t.Run("when docked", func(t *testing.T) {
		t.Run("opens a new chat beside the page from the toggle", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			h := newHarness(t)
			h.Open("/")
			before, err := h.Page.Eval(`() => document.querySelector("[data-slot=app-main]").getBoundingClientRect().width`)
			r.NoError(err)
			h.Click("[data-chat-rail-toggle]")

			h.ElementEventuallyPresent(railPrompt)
			h.EventuallyTrue(`(before) => Math.abs(document.querySelector("[data-slot=app-main]").getBoundingClientRect().width - (before - 400)) < 2`, before.Value.Num())
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "expanded"`)
			h.TextEventuallyContains("#mail-chat-rail-content", "New Chat")
			a.True(h.EvalBool(`() => new URLSearchParams(location.search).get("chat") === "new"`))
		})

		t.Run("starts a thread and streams the reply", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/?chat=new")
			h.ElementEventuallyPresent(railPrompt)
			h.Type(railPrompt, "hello rail")
			h.Press(railPrompt, "Enter")

			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "hello rail")
			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "You said: hello rail")
			h.EventuallyTrue(`() => new URLSearchParams(location.search).get("chat") === "1"`)
			h.ChatThreadCount(1)
		})

		t.Run("opens a thread from a deep link", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("carry me along")
			h.Open(fmt.Sprintf("/inbox/tasks?chat=%d", threadID))

			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "carry me along")
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "expanded"`)
		})

		t.Run("continues an existing thread", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("first")
			h.Open(fmt.Sprintf("/?chat=%d", threadID))
			h.ElementEventuallyPresent(railPrompt)
			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "You said: first")
			h.Type(railPrompt, "second")
			h.Press(railPrompt, "Enter")

			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "You said: second")
		})

		t.Run("opens a chat page link in the rail", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("from the list")
			h.Open(fmt.Sprintf("/inbox/chats/%d", threadID))

			h.PathEventuallyEquals("/inbox/chats")
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "from the list")
		})

		t.Run("closes from the toggle", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/?chat=new")
			h.ElementEventuallyPresent(railPrompt)
			h.Click("[data-chat-rail-toggle]")

			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "collapsed"`)
			h.EventuallyTrue(`() => !new URLSearchParams(location.search).has("chat")`)
		})

		t.Run("keeps the chat open while the left nav changes pages", func(t *testing.T) {
			a := assert.New(t)
			h := newHarness(t)
			threadID := h.SeedRailThread("stay with me")
			h.Open(fmt.Sprintf("/?chat=%d", threadID))
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "stay with me")
			h.Click(`#mail-sidebar a[href="/inbox/tasks"]`)

			h.PathEventuallyEquals("/inbox/tasks")
			h.EventuallyTrue(`() => document.querySelector('#mail-sidebar a[href="/inbox/tasks"]').getAttribute("aria-current") === "page"`)
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "stay with me")
			a.True(h.EvalBool(`(id) => new URLSearchParams(location.search).get("chat") === id`, fmt.Sprint(threadID)))
		})

		t.Run("creates a task in the main section with the chat open", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("beside the task")
			h.Open(fmt.Sprintf("/inbox/tasks?chat=%d", threadID))
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "beside the task")
			h.Click(`[aria-label="New task"]`)

			h.TaskCount(1)
			h.TextEventuallyContains(`[data-mail-content]`, "New task")
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "beside the task")
		})

		t.Run("starts a new chat from the rail header", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("old thread")
			h.Open(fmt.Sprintf("/?chat=%d", threadID))
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "old thread")
			h.Click(`#mail-chat-rail-content [data-chat-new]`)

			h.EventuallyTrue(`() => new URLSearchParams(location.search).get("chat") === "new"`)
			h.EventuallyTrue(`() => !document.querySelector('#mail-chat-rail-content article[aria-label="You"]')`)
			h.Type(railPrompt, "fresh start")
			h.Press(railPrompt, "Enter")
			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "You said: fresh start")
			h.ChatThreadCount(2)
		})

		t.Run("reopens the last chat from the toggle", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("remember me")
			h.Open(fmt.Sprintf("/?chat=%d", threadID))
			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "remember me")
			h.Click("[data-chat-rail-toggle]")
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "collapsed"`)
			h.Click("[data-chat-rail-toggle]")

			h.TextEventuallyContains(`#mail-chat-rail-content article[aria-label="You"]`, "remember me")
			h.EventuallyTrue(`(id) => new URLSearchParams(location.search).get("chat") === id`, fmt.Sprint(threadID))
		})

		t.Run("closes and reopens with back and forward", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-chat-rail-toggle]")
			h.ElementEventuallyPresent(railPrompt)
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "expanded"`)
			h.Page.MustNavigateBack()

			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "collapsed"`)
			h.Page.MustNavigateForward()
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").getAttribute("data-docked") === "expanded"`)
		})

		t.Run("renders thinking, tool, plan, and markdown parts", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.SeedRailThreadMessage("show your work")
			h.SeedAssistantMessage("",
				[2]string{chat.EventThinking, `{"text":"pondering the layout"}`},
				[2]string{chat.EventToolCall, `{"toolCallId":"tc-1","title":"Read main.go","status":"in_progress"}`},
				[2]string{chat.EventToolCallUpdate, `{"toolCallId":"tc-1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"package main"}}]}`},
				[2]string{chat.EventPlan, `{"entries":[{"content":"ship it","status":"in_progress"}]}`},
				[2]string{chat.EventDelta, `{"text":"Done with **bold** work"}`},
			)
			h.Open("/?chat=1")
			h.ElementEventuallyPresent("#mail-chat-rail-content [data-slot=disclosure] > summary")
			h.Click("#mail-chat-rail-content [data-slot=disclosure] > summary")

			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "Read main.go")
			h.EventuallyTrue(`() => document.querySelector("#mail-chat-rail-content [data-chat-entries]").textContent.includes("ship it")`)
			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries] strong", "bold")
		})

		t.Run("stops a streaming turn", func(t *testing.T) {
			h := newHarness(t)
			h.SeedChatThread("e2e chat")
			h.SeedRailThreadMessage("keep going")
			h.SeedStreamingAssistant("Working")
			h.Open("/?chat=1")
			h.ElementEventuallyPresent(`#mail-chat-rail-content [aria-label="Stop"]`)
			h.Click(`#mail-chat-rail-content [aria-label="Stop"]`)

			h.TextEventuallyContains("#mail-chat-rail-content [data-chat-entries]", "The turn was stopped.")
		})
	})

	t.Run("when narrow", func(t *testing.T) {
		t.Run("opens the left nav from the bottom bar and navigates", func(t *testing.T) {
			h := newHarness(t)
			h.Page.MustSetViewport(390, 844, 1, false)
			h.Open("/")
			h.Click(`[aria-controls="mail-sidebar"][data-drawer-trigger]`)

			h.EventuallyTrue(`() => document.querySelector("#mail-sidebar").hasAttribute("data-drawer-open")`)
			h.Click(`#mail-sidebar a[href="/inbox/tasks"]`)
			h.PathEventuallyEquals("/inbox/tasks")
			h.EventuallyTrue(`() => !document.querySelector("#mail-sidebar").hasAttribute("data-drawer-open")`)
		})

		t.Run("switches between the page and the chat from the bottom bar", func(t *testing.T) {
			h := newHarness(t)
			h.Page.MustSetViewport(390, 844, 1, false)
			h.Open("/")
			h.Click(`[data-shell-switch="chat"]`)

			h.ElementEventuallyPresent(railPrompt)
			h.EventuallyTrue(`() => document.querySelector("[data-chat-rail]").hasAttribute("data-drawer-open")`)
			h.Click(`[data-shell-switch="mail"]`)
			h.EventuallyTrue(`() => !document.querySelector("[data-chat-rail]").hasAttribute("data-drawer-open")`)
		})
	})
}
