package inbox

import (
	"testing"
	"time"

	"github.com/housecat-inc/scratch/testkit"
)

func evalBool(h *Harness, js string) bool {
	v, err := h.Page.Eval(js)
	return err == nil && v.Value.Bool()
}

func markReader() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Page.Eval(`() => { document.querySelector('#wf-reader').dataset.pollTest = '1'; return true }`)
		h.R.NoError(err)
	}
}

func selectReaderText() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Page.Eval(`() => {
			const el = document.querySelector('#wf-body');
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

func clearSelection() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		_, err := h.Page.Eval(`() => { window.getSelection().removeAllRanges(); return true }`)
		h.R.NoError(err)
	}
}

func readerMarkerStable() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Never(func() bool {
			return !evalBool(h, `() => !!document.querySelector('#wf-reader[data-poll-test]')`)
		}, 1500*time.Millisecond, 150*time.Millisecond)
	}
}

func readerMarkerCleared() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			return !evalBool(h, `() => !!document.querySelector('#wf-reader[data-poll-test]')`)
		}, 3*time.Second, 100*time.Millisecond)
	}
}

func TestWorkflowPollPausesDuringTextSelection(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				SelectOption(`[name="workflow_type"]`, "webhook"),
				TextContains("#wf-body", "Waiting for an external callback"),
				markReader(),
				selectReaderText(),
			},
			Assert: []Step{
				readerMarkerStable(),
				clearSelection(),
				readerMarkerCleared(),
			},
			Name: "auto-refresh pauses while text is selected then resumes",
			Path: "/",
		},
	})
}
