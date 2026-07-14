package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPopoutExpandFromMinimizedRestoresComposer(t *testing.T) {
	var threadID int64
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Seed: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					thread, err := h.Chat.CreateThread("", "")
					h.R.NoError(err)
					_, err = h.Chat.Send(thread.ID, "hello there")
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
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
			},
			Name: "expanding a minimized popout restores a usable composer",
			Path: "/inbox/chats",
		},
	})
}
