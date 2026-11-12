package inbox

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestScheduleBrowser(t *testing.T) {
	t.Run("lists the heartbeat schedule as an active row", func(t *testing.T) {
		h := newHarness(t)
		h.Open("/inbox/workflows")
		h.SeedSchedule()
		h.Load("/inbox/workflows")
		h.Hover("[data-id=heartbeat]")

		h.ElementAbsent("[data-id=heartbeat] .gm-row-kind-icon")
		h.ElementVisible("[data-id=heartbeat] .gm-star")
		h.ElementTextContains("[data-id=heartbeat] .gm-row-label", "Scheduled")
		h.ElementTextContains("[data-id=heartbeat] .gm-row-labels", "Active")
		h.ElementTextContains("[data-id=heartbeat] .gm-row-subject-text", "0 * * * * *")
		h.ElementTextContains("[data-id=heartbeat] .gm-row-what", "heartbeat")
		h.ElementVisible("[data-schedule-btn=pause]")
	})

	t.Run("pausing a schedule flips its label to paused", func(t *testing.T) {
		h := newHarness(t)
		h.Open("/inbox/workflows")
		h.SeedSchedule()
		h.Load("/inbox/workflows")
		h.Hover("[data-id=heartbeat]")
		h.Click("[data-schedule-btn=pause]")
		h.Load("/inbox/workflows")
		h.Hover("[data-id=heartbeat]")

		h.ElementTextContains("[data-id=heartbeat] .gm-row-labels", "Paused")
		h.ElementVisible("[data-schedule-btn=resume]")
	})

	t.Run("opening a schedule shows its execution feed scrolled to the bottom", func(t *testing.T) {
		r := require.New(t)
		h := newHarness(t)
		h.Open("/inbox/workflows")
		h.SeedSchedule()
		h.SeedScheduleRuns(12)
		h.Load("/inbox/workflows")
		h.Click("[data-id=heartbeat] .gm-row-link")

		h.ElementTextContains(".mail-reader-title h1", "heartbeat")
		h.ElementTextContains(".mail-reader-labels", "Scheduled")
		h.ElementTextContains("#wf-body", "Execution")
		h.ElementTextContains("#wf-body", "beat at")
		r.Eventually(func() bool {
			return h.EvalBool(`() => {
				const scroller = document.querySelector(".mail-chat-body");
				if (!scroller) return false;
				return scroller.scrollHeight - scroller.scrollTop - scroller.clientHeight < 4;
			}`)
		}, 2*time.Second, 50*time.Millisecond)
	})
}
