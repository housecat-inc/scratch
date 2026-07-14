package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPopoutClickMinimizeCollapses(t *testing.T) {
	var threadID int64
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Seed: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					thread, err := h.Chat.CreateThread("", "")
					h.R.NoError(err)
					_, err = h.Chat.Send(thread.ID, "Ping")
					h.R.NoError(err)
					threadID = thread.ID
				},
			},
			Act: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					_, err := h.Page.Eval(`(id) => sessionStorage.setItem('scratch.chat.popout', JSON.stringify({mode:'open', threadID: String(id)}))`, threadID)
					h.R.NoError(err)
					h.Load("/inbox/chats")
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => document.getElementById('floating-chat') !== null`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
				Click("[data-chat-minimize]"),
			},
			Assert: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => {
							const panel = document.getElementById('floating-chat');
							if (!panel || !panel.classList.contains('minimized')) return false;
							const rows = getComputedStyle(panel).gridTemplateRows.split(' ');
							return Math.round(panel.getBoundingClientRect().height) <= 60 &&
								parseFloat(rows[1]) === 0 &&
								parseFloat(rows[2]) === 0;
						}`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
			},
			Name: "clicking minimize collapses the popout to its header",
			Path: "/inbox/chats",
		},
	})
}
