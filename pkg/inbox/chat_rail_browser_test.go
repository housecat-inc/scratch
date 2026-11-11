package inbox

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const longDescription = "This should look like a finished / disabled form but have editable fields and a really really long description that keeps going and going"

func TestChatRailBrowser(t *testing.T) {
	t.Run("when docked", func(t *testing.T) {
		t.Run("opens beside the page and narrows the main column", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			h := newHarness(t)
			h.Open("/")
			before, err := h.Page.Eval(`() => document.querySelector(".mail-main").getBoundingClientRect().width`)
			r.NoError(err)

			h.Click("[data-chat-rail-toggle]")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
			h.EventuallyTrue(`(before) => Math.abs(document.querySelector(".mail-main").getBoundingClientRect().width - (before - 400)) < 2`, before.Value.Num())
			h.ElementAttributeEquals("[data-chat-rail-toggle]", "aria-expanded", "true")
			a.True(h.EvalBool(`() => document.documentElement.dataset.chatRail === "open"`))
		})

		t.Run("sizes the composer to one line when opened from the toggle", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-chat-rail-toggle]")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")

			h.EventuallyTrue(`() => document.querySelector("#chat-rail-panel [data-chat-input]").getBoundingClientRect().height < 60`)
		})

		t.Run("keeps the chat open across navigation", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("carry me along")
			h.Open("/")
			h.SetRailCookie(true, threadID)
			h.Load("/inbox/tasks")

			h.ElementEventuallyPresent(fmt.Sprintf(`#chat-rail-panel[data-thread-id="%d"]`, threadID))
			h.TextEventuallyContains("#chat-rail-panel .role-user", "carry me along")
		})

		t.Run("streams a reply into the panel", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread()
			h.Open("/")
			h.SetRailCookie(true, threadID)
			h.Load("/inbox/tasks")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
			h.Type("#chat-rail-panel [data-chat-input]", "from the rail")
			h.Click("#chat-rail-panel [data-chat-send]")

			h.TextEventuallyContains("#chat-rail-panel .role-user .chat-bubble", "from the rail")
			h.TextEventuallyContains("#chat-rail-panel .role-assistant", "You said: from the rail")
		})

		t.Run("closes from the header and stays closed after reload", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("hello")
			h.Open("/")
			h.SetRailCookie(true, threadID)
			h.Load("/")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-rail-close]")

			h.Click("#chat-rail-panel [data-chat-rail-close]")
			h.ElementAbsent("#chat-rail-panel")
			h.Load("/")
			h.ElementAbsent("#chat-rail-panel")
			h.ElementAttributeEquals("html", "data-chat-rail", "closed")
		})

		t.Run("reopens the last chat from the toggle", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("remember me")
			h.Open("/")
			h.SetRailCookie(false, threadID)
			h.Load("/")
			h.ElementAbsent("#chat-rail-panel")

			h.Click("[data-chat-rail-toggle]")
			h.ElementEventuallyPresent(fmt.Sprintf(`#chat-rail-panel[data-thread-id="%d"]`, threadID))
		})

		t.Run("opens a deep linked chat from the chat query", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("linked")
			h.Open(fmt.Sprintf("/inbox/tasks?chat=%d", threadID))

			h.ElementEventuallyPresent(fmt.Sprintf(`#chat-rail-panel[data-thread-id="%d"]`, threadID))
		})

		t.Run("discards an empty new chat on close", func(t *testing.T) {
			h := newHarness(t)
			h.Open("/")
			h.Click("[data-chat-rail-toggle]")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-rail-close]")
			h.ChatThreadCount(1)

			h.Click("#chat-rail-panel [data-chat-rail-close]")
			h.ChatThreadCount(0)
		})

		t.Run("keeps the header actions visible with a long description", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread(longDescription)
			h.Open("/")
			h.SetRailCookie(true, threadID)
			h.Load("/")

			h.EventuallyTrue(`() => {
				const panel = document.getElementById("chat-rail-panel");
				if (!panel) return false;
				const close = panel.querySelector("[data-chat-rail-close]").getBoundingClientRect();
				const pr = panel.getBoundingClientRect();
				return close.width > 0 && close.right <= pr.right + 1 && close.right <= window.innerWidth;
			}`)
		})
	})

	t.Run("when narrow", func(t *testing.T) {
		t.Run("covers the page and closes on escape", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedRailThread("on the go")
			h.Viewport(390, 800)
			h.Open("/")
			h.SetRailCookie(true, threadID)
			h.Load("/")
			h.ElementEventuallyPresent("#chat-rail-panel [data-chat-input]")
			h.EventuallyTrue(`() => Math.abs(document.getElementById("chat-rail").getBoundingClientRect().width - window.innerWidth) < 1`)

			h.Press("#chat-rail-panel [data-chat-input]", "Escape")
			h.ElementAbsent("#chat-rail-panel")
		})
	})
}
