package inbox

import (
	"fmt"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPopoutRestoreScrollsToBottom(t *testing.T) {
	var threadID int64
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Seed: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					thread, err := h.Chat.CreateThread("", "")
					h.R.NoError(err)
					threadID = thread.ID
					for i := range 8 {
						_, err = h.Chat.Send(thread.ID, fmt.Sprintf("Message number %d in a fairly long conversation", i))
						h.R.NoError(err)
						time.Sleep(30 * time.Millisecond)
					}
					time.Sleep(400 * time.Millisecond)
				},
			},
			Act: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					_, err := h.Page.Eval(`(id) => sessionStorage.setItem('scratch.chat.popout', JSON.stringify({mode:'minimized', threadID: String(id)}))`, threadID)
					h.R.NoError(err)
					h.Load("/inbox/chats")
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => document.getElementById('floating-chat')?.classList.contains('minimized') === true`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
				Click("[data-chat-restore]"),
			},
			Assert: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => {
							const panel = document.getElementById('floating-chat');
							if (!panel || panel.classList.contains('minimized')) return false;
							const s = panel.querySelector('[data-chat-scroller]');
							if (!s || s.clientHeight === 0 || s.scrollHeight <= s.clientHeight) return false;
							return s.scrollHeight - s.scrollTop - s.clientHeight < 40;
						}`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
			},
			Name: "restoring a minimized popout scrolls to the latest message",
			Path: "/inbox/chats",
		},
	})
}
