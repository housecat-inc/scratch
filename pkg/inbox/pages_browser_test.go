package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPagesBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act:    []Step{Click(`button[aria-label="Pin Example Page as home"]`), Load("/")},
			Assert: []Step{Visible(`.page-header`), TextContains(`.page-header`, "Pinned Home")},
			Name:   "pin home changes the landing page",
			Path:   "/pages",
		},
		{
			Act: []Step{Click(`.page-header [data-chat-popout-new] button`)},
			Assert: []Step{
				Visible(`#floating-chat`),
				Visible(`.page-header`),
				func(t *testing.T, h *Harness) {
					page, err := h.Store.GetPage("example")
					h.R.NoError(err)
					h.R.Zero(page.ThreadID)
				},
			},
			Name: "edit opens a normal popup chat",
			Path: "/pages/example",
		},
		{
			Act: []Step{Click(`button[aria-label="Unpin Example Page from sidebar"]`)},
			Assert: []Step{
				ElementAbsent(`.mail-sidebar a[href="/pages/example"]`),
				Visible(`.page-card a[href="/pages/example"]`),
				Visible(`button[aria-label="Pin Example Page to sidebar"]`),
				func(t *testing.T, h *Harness) { h.Screenshot("pages.png") },
			},
			Name: "unpin keeps the page in the library",
			Path: "/pages",
		},
		{
			Act: []Step{func(t *testing.T, h *Harness) {
				h.R.NoError(h.Store.SetPagePinned("example", false))
			}, Load("/pages"), Click(`button[aria-label="Pin Example Page to sidebar"]`)},
			Assert: []Step{Visible(`.mail-sidebar a[href="/pages/example"]`)},
			Name:   "pin restores the sidebar shortcut",
			Path:   "/pages",
		},
	})
}
