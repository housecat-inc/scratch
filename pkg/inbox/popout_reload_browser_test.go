package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPopoutReloadRestoresCollapsedMinimized(t *testing.T) {
	var threadID int64
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Seed: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					thread, err := h.Chat.CreateThread("", "")
					h.R.NoError(err)
					_, err = h.Chat.Send(thread.ID, "This should look like a finished / disabled form but have editable fields and a really really long description that keeps going and going")
					h.R.NoError(err)
					threadID = thread.ID
				},
			},
			Act: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					_, err := h.Page.Eval(`(id) => sessionStorage.setItem('scratch.chat.popout', JSON.stringify({mode:'minimized', threadID: String(id)}))`, threadID)
					h.R.NoError(err)
					h.Load("/inbox/chats")
				},
			},
			Assert: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => {
							const panel = document.getElementById('floating-chat');
							if (!panel || !panel.classList.contains('minimized')) return false;
							const close = panel.querySelector('[data-chat-close]');
							const pr = panel.getBoundingClientRect();
							const cr = close.getBoundingClientRect();
							const rows = getComputedStyle(panel).gridTemplateRows.split(' ');
							return cr.width > 0 &&
								cr.right <= pr.right + 1 &&
								cr.right <= window.innerWidth &&
								pr.width <= 360 &&
								parseFloat(rows[1]) === 0 &&
								parseFloat(rows[2]) === 0;
						}`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
			},
			Name: "reload restores a fully collapsed minimized popout with visible actions",
			Path: "/inbox/chats",
		},
	})
}
