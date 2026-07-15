package inbox

import (
	"strings"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/testkit"
)

func evalBool(h *Harness, js string) bool {
	v, err := h.Page.Eval(js)
	return err == nil && v.Value.Bool()
}

func markFirstStep() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Page.Eval(`() => { document.querySelector('#wf-steps > div').dataset.sseTest = '1'; return true }`)
		h.R.NoError(err)
	}
}

func selectFirstStepText() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Page.Eval(`() => {
			const el = document.querySelector('#wf-steps > div .chat-row-label');
			const range = document.createRange();
			range.selectNodeContents(el);
			const sel = window.getSelection();
			sel.removeAllRanges();
			sel.addRange(range);
			return true;
		}`)
		h.R.NoError(err)
	}
}

func firstStepMarkerStable() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Never(func() bool {
			return !evalBool(h, `() => !!document.querySelector('#wf-steps > div[data-sse-test]')`)
		}, 1500*time.Millisecond, 150*time.Millisecond)
	}
}

func selectionContainsStable(want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Never(func() bool {
			v, err := h.Page.Eval(`() => window.getSelection().toString()`)
			return err != nil || !strings.Contains(v.Value.Str(), want)
		}, 1500*time.Millisecond, 150*time.Millisecond)
	}
}

func TestWorkflowLiveUpdatesPreserveEarlierSteps(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "countdown"),
				TextContains("#wf-steps", "T-minus 3"),
				markFirstStep(),
				selectFirstStepText(),
			},
			Assert: []Step{
				TextContains("#wf-steps", "T-minus 2"),
				firstStepMarkerStable(),
				selectionContainsStable("T-minus 3"),
			},
			Name: "appended steps never replace earlier ones or drop the user's selection",
			Path: "/",
		},
	})
}
