package inbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPollBrowser(t *testing.T) {
	t.Run("appends later workflow steps without re-rendering earlier ones", func(t *testing.T) {
		r := require.New(t)
		h := newHarness(t)
		h.Open("/")
		h.SelectOption(`[name="workflow_type"]`, "countdown")
		h.ElementTextContains("#wf-steps", "T-minus 3")
		_, err := h.Page.Eval(`() => { document.querySelector('#wf-steps > div').dataset.sseTest = '1'; return true }`)
		r.NoError(err)

		h.ElementTextContains("#wf-steps", "T-minus 2")
		r.Never(func() bool {
			return !h.EvalBool(`() => !!document.querySelector('#wf-steps > div[data-sse-test]')`)
		}, 1500*time.Millisecond, 150*time.Millisecond)
	})
}
