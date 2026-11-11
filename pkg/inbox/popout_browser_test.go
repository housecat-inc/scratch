package inbox

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const longDescription = "This should look like a finished / disabled form but have editable fields and a really really long description that keeps going and going"

func TestPopoutBrowser(t *testing.T) {
	t.Run("when minimized", func(t *testing.T) {
		t.Run("keeps the action icons visible with a long description", func(t *testing.T) {
			r := require.New(t)
			h := newHarness(t)
			threadID := h.SeedPopoutThread(longDescription)
			h.Open("/")
			_, err := h.Page.Eval(`async (id) => {
				const html = await (await fetch('/chat/' + id + '/popout')).text();
				document.body.insertAdjacentHTML('beforeend', html);
				document.getElementById('floating-chat').classList.add('minimized');
			}`, threadID)
			r.NoError(err)

			h.EventuallyTrue(`() => {
				const panel = document.getElementById('floating-chat');
				const close = panel.querySelector('[data-chat-close]');
				const pr = panel.getBoundingClientRect();
				const cr = close.getBoundingClientRect();
				return cr.width > 0 &&
					cr.right <= pr.right + 1 &&
					cr.right <= window.innerWidth &&
					pr.width <= 360;
			}`)
		})

		t.Run("stays fully collapsed with visible actions after reload", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedPopoutThread(longDescription)
			h.Open("/inbox/chats")
			h.SetPopoutState("minimized", threadID)
			h.Load("/inbox/chats")

			h.EventuallyTrue(`() => {
				const panel = document.getElementById('floating-chat');
				if (!panel || !panel.classList.contains('minimized')) return false;
				const close = panel.querySelector('[data-chat-close]');
				const pr = panel.getBoundingClientRect();
				const cr = close.getBoundingClientRect();
				const head = panel.querySelector('.floating-chat-head').getBoundingClientRect();
				let leaks = 0;
				panel.querySelectorAll('*').forEach(el => {
					const r = el.getBoundingClientRect();
					if (r.height > 0.5 && (r.bottom > pr.bottom + 0.5 || r.top < pr.top - 0.5)) leaks++;
				});
				return cr.width > 0 &&
					cr.right <= pr.right + 1 &&
					cr.right <= window.innerWidth &&
					pr.width <= 360 &&
					Math.abs(head.bottom - pr.bottom) <= 1 &&
					leaks === 0;
			}`)
		})
	})

	t.Run("minimize", func(t *testing.T) {
		t.Run("collapses the popout to its header", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedPopoutThread("Ping")
			h.Open("/inbox/chats")
			h.SetPopoutState("open", threadID)
			h.Load("/inbox/chats")
			h.EventuallyTrue(`() => document.getElementById('floating-chat') !== null`)
			h.Click("[data-chat-minimize]")

			h.EventuallyTrue(`() => {
				const panel = document.getElementById('floating-chat');
				if (!panel || !panel.classList.contains('minimized')) return false;
				const rows = getComputedStyle(panel).gridTemplateRows.split(' ');
				return Math.round(panel.getBoundingClientRect().height) <= 60 &&
					parseFloat(rows[1]) === 0 &&
					parseFloat(rows[2]) === 0;
			}`)
		})
	})

	t.Run("restore", func(t *testing.T) {
		t.Run("brings back a usable composer", func(t *testing.T) {
			h := newHarness(t)
			threadID := h.SeedPopoutThread("hello there")
			h.Open("/inbox/chats")
			h.SetPopoutState("minimized", threadID)
			h.Load("/inbox/chats")
			h.EventuallyTrue(`() => document.getElementById('floating-chat')?.classList.contains('minimized') === true`)
			h.Click("[data-chat-restore]")

			h.EventuallyTrue(`() => {
				const panel = document.getElementById('floating-chat');
				if (!panel || panel.classList.contains('minimized')) return false;
				const input = panel.querySelector('[data-chat-input]');
				const compose = panel.querySelector('.chat-compose');
				if (!input || !compose) return false;
				const ir = input.getBoundingClientRect();
				const pr = panel.getBoundingClientRect();
				const cr = compose.getBoundingClientRect();
				return ir.height >= 20 &&
					cr.left >= pr.left - 0.5 &&
					cr.right <= pr.right + 0.5 &&
					cr.bottom <= pr.bottom + 0.5;
			}`)
		})

		t.Run("scrolls to the latest message", func(t *testing.T) {
			r := require.New(t)
			h := newHarness(t)
			thread, err := h.Chat.CreateThread("", "")
			r.NoError(err)
			for i := range 8 {
				_, err = h.Chat.Send(thread.ID, fmt.Sprintf("Message number %d in a fairly long conversation", i))
				r.NoError(err)
				time.Sleep(30 * time.Millisecond)
			}
			time.Sleep(400 * time.Millisecond)
			h.Open("/inbox/chats")
			h.SetPopoutState("minimized", thread.ID)
			h.Load("/inbox/chats")
			h.EventuallyTrue(`() => document.getElementById('floating-chat')?.classList.contains('minimized') === true`)
			h.Click("[data-chat-restore]")

			h.EventuallyTrue(`() => {
				const panel = document.getElementById('floating-chat');
				if (!panel || panel.classList.contains('minimized')) return false;
				const s = panel.querySelector('[data-chat-scroller]');
				if (!s || s.clientHeight === 0 || s.scrollHeight <= s.clientHeight) return false;
				return s.scrollHeight - s.scrollTop - s.clientHeight < 40;
			}`)
		})
	})
}
