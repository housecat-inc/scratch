package inbox

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-rod/rod/lib/proto"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/testkit"
)

func SeedChatThread() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Chat.CreateThread("", "e2e chat")
		h.R.NoError(err)
	}
}

func SeedChatThreadTitle(title string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Chat.CreateThread("", title)
		h.R.NoError(err)
	}
}

func SeedAssistantMessage(body string, events ...[2]string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		msg, err := h.Store.AddMessage(db.NewMessage{
			Author:   "agent:echo",
			Body:     body,
			Role:     db.MessageRoleAssistant,
			ThreadID: 1,
		})
		h.R.NoError(err)
		for _, ev := range events {
			_, err := h.Store.AddMessageEvent(msg.ID, ev[0], ev[1])
			h.R.NoError(err)
		}
	}
}

func SeedStreamingAssistant(body string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Store.AddMessage(db.NewMessage{
			Author:   "agent:echo",
			Body:     body,
			Role:     db.MessageRoleAssistant,
			Status:   db.MessageStatusStreaming,
			ThreadID: 1,
		})
		h.R.NoError(err)
	}
}

func AttachFile(name, contents string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		path := filepath.Join(t.TempDir(), name)
		h.R.NoError(os.WriteFile(path, []byte(contents), 0o644))
		h.Page.MustElement("#chat-file").MustSetFiles(path)
	}
}

func AttributeEquals(selector, name, expected string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.ElementAttributeEquals(selector, name, expected)
	}
}

func InputValueContains(selector, expected string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventuallyf(func() bool {
			el, err := h.Page.Element(selector)
			return err == nil && strings.Contains(el.MustProperty("value").Str(), expected)
		}, 20*time.Second, 50*time.Millisecond, "%s value should contain %q", selector, expected)
	}
}

func PathEventuallyEquals(expected string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventuallyf(func() bool {
			result, err := h.Page.Eval(`() => location.pathname`)
			return err == nil && result.Value.Str() == expected
		}, 20*time.Second, 50*time.Millisecond, "path should equal %q", expected)
	}
}

func ElementEventuallyPresent(selector string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventuallyf(func() bool {
			_, err := h.Page.Element(selector)
			return err == nil
		}, 20*time.Second, 50*time.Millisecond, "%s should be present", selector)
	}
}

func FloatingChatControlsFit() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			result, err := h.Page.Eval(`() => {
				const panel = document.querySelector("#floating-chat");
				const close = document.querySelector("#floating-chat [data-chat-close]");
				const expand = document.querySelector("#floating-chat [data-chat-fullscreen]");
				const minimize = document.querySelector("#floating-chat [data-chat-minimize]");
				const restore = document.querySelector("#floating-chat [data-chat-restore]");
				const access = document.querySelector("#floating-chat .chat-access-switch");
				if (!panel || !close || !expand || !minimize || !restore || !access) return false;
				const panelRect = panel.getBoundingClientRect();
				const visible = (el) => {
					const style = getComputedStyle(el);
					const rect = el.getBoundingClientRect();
					return style.display !== "none" && style.visibility !== "hidden" && rect.width > 0 && rect.height > 0;
				};
				const inside = (el) => {
					const rect = el.getBoundingClientRect();
					return rect.left >= panelRect.left && rect.right <= panelRect.right && rect.top >= panelRect.top && rect.bottom <= panelRect.bottom;
				};
				return !visible(access) && !visible(minimize) && visible(close) && visible(expand) && visible(restore) && inside(close) && inside(expand) && inside(restore);
			}`)
			return err == nil && result.Value.Bool()
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func TextEventuallyContains(selector, expected string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventuallyf(func() bool {
			el, err := h.Page.Element(selector)
			if err != nil {
				return false
			}
			text, err := el.Text()
			return err == nil && strings.Contains(text, expected)
		}, 20*time.Second, 50*time.Millisecond, "%s text should contain %q", selector, expected)
	}
}

func CaptureElement(selector string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.Page.MustElement("#chat-snap").MustClick()
		el := h.Page.MustElement(selector)
		box := el.MustShape().Box()
		x := box.X + box.Width/2
		y := box.Y + box.Height/2
		h.Page.Mouse.MustMoveTo(x, y)
		h.ElementVisible("#chat-select-overlay")
		h.Page.Mouse.MustClick(proto.InputMouseButtonLeft)
	}
}

func WaitChatReady() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			result, err := h.Page.Eval(`() => Boolean(window.htmx && document.querySelector("#chat-form"))`)
			return err == nil && result.Value.Bool()
		}, 20*time.Second, 50*time.Millisecond)
	}
}

func TestChatBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Name: "inspects a page section before creating a chat",
			Path: "/",
			Act: []Step{
				WaitChatReady(),
				CaptureElement(".mail-mainbar"),
				TextEventuallyContains("#chat-attachments", "selection.png"),
				TextEventuallyContains("#chat-attachments", "selection.html"),
				Click("#chat-send"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.png"),
				TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.html"),
				TextEventuallyContains("#chat-messages .role-user .chat-bubble", "Selected"),
			},
		},
		{
			Name: "inspector on a task page starts a chat",
			Path: "/inbox/tasks/1",
			Seed: []Step{
				SeedTask("broken task"),
			},
			Act: []Step{
				WaitChatReady(),
				CaptureElement(".mail-reader-title"),
				TextEventuallyContains("#chat-attachments", "selection.png"),
				TextEventuallyContains("#chat-attachments", "selection.html"),
				Click("#chat-send"),
			},
			Assert: []Step{
				ChatThreadCount(1),
				TaskCount(1),
				TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.png"),
				TextEventuallyContains("#chat-messages .chat-bubble-attachments", "selection.html"),
				TextEventuallyContains("#chat-messages .role-user .chat-bubble", "Selected"),
			},
		},
		{
			Name: "sends a message and streams the echoed reply",
			Path: "/inbox/chats/1",
			Seed: []Step{SeedChatThread()},
			Act: []Step{
				WaitChatReady(),
				Type("#chat-input", "hello e2e"),
				Click("#chat-send"),
			},
			Assert: []Step{
				TextEventuallyContains("#chat-messages .role-user .chat-bubble", "hello e2e"),
				TextEventuallyContains("#chat-messages .role-assistant", "You said: hello e2e"),
				func(t *testing.T, h *Harness) {
					t.Helper()
					h.A.Equal("", h.Page.MustElement("#chat-input").MustProperty("value").Str())
				},
			},
		},
		{
			Name: "opens an existing thread in a floating chat",
			Path: "/inbox/chats/1",
			Seed: []Step{SeedChatThread()},
			Act: []Step{
				WaitChatReady(),
				Click(`[data-chat-popout-thread="1"]`),
				PathEventuallyEquals("/inbox/chats"),
				ElementEventuallyPresent("#floating-chat"),
				Type("#floating-chat [data-chat-input]", "from popout"),
				Click("#floating-chat [data-chat-send]"),
			},
			Assert: []Step{
				TextEventuallyContains("#floating-chat .role-user .chat-bubble", "from popout"),
				TextEventuallyContains("#floating-chat .role-assistant", "You said: from popout"),
			},
		},
		{
			Name: "keeps minimized floating chat actions visible",
			Path: "/inbox/chats/1",
			Seed: []Step{
				SeedChatThreadTitle(strings.Repeat("long thread title ", 20)),
				SeedAssistantMessage(strings.Repeat("long thread content ", 120)),
			},
			Act: []Step{
				WaitChatReady(),
				Click(`[data-chat-popout-thread="1"]`),
				ElementEventuallyPresent("#floating-chat"),
				Click("#floating-chat [data-chat-minimize]"),
				FloatingChatControlsFit(),
				Click("#floating-chat [data-chat-restore]"),
			},
			Assert: []Step{
				Visible("#floating-chat [data-chat-input]"),
			},
		},
		{
			Name: "renders thinking, tool, plan, and markdown parts",
			Path: "/inbox/chats/1",
			Seed: []Step{
				SeedChatThread(),
				SeedAssistantMessage("",
					[2]string{chat.EventThinking, `{"text":"pondering the layout"}`},
					[2]string{chat.EventToolCall, `{"toolCallId":"tc-1","title":"Read main.go","status":"in_progress"}`},
					[2]string{chat.EventToolCallUpdate, `{"toolCallId":"tc-1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"package main"}}]}`},
					[2]string{chat.EventPlan, `{"entries":[{"content":"ship it","status":"in_progress"}]}`},
					[2]string{chat.EventDelta, `{"text":"Done with **bold** work"}`},
				),
			},
			Act: []Step{
				WaitChatReady(),
				Click(".chat-row.row-tool summary"),
			},
			Assert: []Step{
				TextEventuallyContains(".chat-row.row-thinking summary", "Thinking"),
				TextEventuallyContains(".chat-row.row-tool .chat-row-label", "Read main.go"),
				TextEventuallyContains(".chat-row.row-tool .chat-row-detail", "package main"),
				TextEventuallyContains(".chat-plan", "ship it"),
				TextEventuallyContains(".chat-md strong", "bold"),
			},
		},
		{
			Name: "uploads an attachment and sends it as a pill",
			Path: "/inbox/chats/1",
			Seed: []Step{SeedChatThread()},
			Act: []Step{
				WaitChatReady(),
				AttachFile("notes.txt", "remember this"),
				TextContains("#chat-attachments .chat-chip-name", "notes.txt"),
				Type("#chat-input", "about the attached file"),
				Click("#chat-send"),
			},
			Assert: []Step{
				TextEventuallyContains("#chat-messages .role-user .chat-bubble", "about the attached file"),
				TextEventuallyContains("#chat-messages .chat-bubble-attachments .chat-chip-name", "notes.txt"),
				TextEventuallyContains("#chat-messages .role-assistant", "You said:"),
				ElementAbsent("#chat-attachments .chat-attachment-chip"),
			},
		},
		{
			Name: "captures a page section as image and html attachments",
			Path: "/inbox/chats/1",
			Seed: []Step{
				SeedChatThread(),
				SeedAssistantMessage("Capture me"),
			},
			Act: []Step{
				WaitChatReady(),
				CaptureElement("#message-1 .chat-md"),
			},
			Assert: []Step{
				TextEventuallyContains("#chat-attachments", "selection.png"),
				TextEventuallyContains("#chat-attachments", "selection.html"),
				InputValueContains("#chat-input", "Selected #message-1"),
				InputValueContains("#chat-input", "/inbox/chats/1"),
			},
		},
		{
			Name: "toggles between full access and safe mode",
			Path: "/inbox/chats/1",
			Seed: []Step{SeedChatThread()},
			Act: []Step{
				WaitChatReady(),
				AttributeEquals(".chat-access-switch", "aria-checked", "true"),
				Click(".chat-access-switch"),
			},
			Assert: []Step{
				AttributeEquals(".chat-access-switch", "aria-checked", "false"),
				func(t *testing.T, h *Harness) {
					t.Helper()
					thread, err := h.Chat.Thread(1)
					h.R.NoError(err)
					h.A.Equal(chat.AccessSafe, h.Chat.ThreadAccess(thread))
				},
			},
		},
		{
			Console: []string{"[[object Event]]"},
			Name:    "stops a streaming chat turn",
			Path:    "/inbox/chats/1",
			Seed: []Step{
				SeedChatThread(),
				SeedStreamingAssistant("Working"),
			},
			Act: []Step{
				WaitChatReady(),
				Click(`[aria-label="Stop chat"]`),
			},
			Assert: []Step{
				TextEventuallyContains("#chat-messages", "The turn was stopped."),
				ElementEventuallyPresent(`[aria-label="Stop chat"]`),
			},
		},
	})
}
