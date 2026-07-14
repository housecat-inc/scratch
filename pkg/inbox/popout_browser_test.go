package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPopoutMinimizedTruncatesDescription(t *testing.T) {
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
					_, err := h.Page.Eval(`async (id) => {
						const html = await (await fetch('/chat/' + id + '/popout')).text();
						document.body.insertAdjacentHTML('beforeend', html);
						document.getElementById('floating-chat').classList.add('minimized');
					}`, threadID)
					h.R.NoError(err)
				},
			},
			Assert: []Step{
				func(t *testing.T, h *Harness) {
					t.Helper()
					h.R.Eventually(func() bool {
						v, err := h.Page.Eval(`() => {
							const panel = document.getElementById('floating-chat');
							const close = panel.querySelector('[data-chat-close]');
							const pr = panel.getBoundingClientRect();
							const cr = close.getBoundingClientRect();
							return cr.width > 0 &&
								cr.right <= pr.right + 1 &&
								cr.right <= window.innerWidth &&
								pr.width <= 360;
						}`)
						return err == nil && v.Value.Bool()
					}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
				},
			},
			Name: "minimized popout keeps action icons visible with a long description",
			Path: "/",
		},
	})
}
