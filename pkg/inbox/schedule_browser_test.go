package inbox

import (
	"testing"
	"time"

	"github.com/housecat-inc/scratch/testkit"
)

func SeedSchedule() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.NoError(h.Flows.EnsureSchedules())
	}
}

func SeedScheduleRun() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		runID, err := h.Flows.TriggerSchedule("heartbeat")
		h.R.NoError(err)
		h.Flows.Await(runID, 5*time.Second)
	}
}

func TestWorkflowSchedulesBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				SeedSchedule(),
				Load("/inbox/workflows"),
				Hover("[data-id=heartbeat]"),
			},
			Assert: []Step{
				TextContains("[data-id=heartbeat] .gm-row-label", "Scheduled"),
				TextContains("[data-id=heartbeat] .gm-row-labels", "Active"),
				TextContains("[data-id=heartbeat] .gm-row-subject-text", "0 * * * * *"),
				TextContains("[data-id=heartbeat] .gm-row-what", "heartbeat"),
				Visible("[data-schedule-btn=pause]"),
			},
			Name: "schedule row lists the heartbeat schedule as active",
			Path: "/inbox/workflows",
		},
		{
			Act: []Step{
				SeedSchedule(),
				Load("/inbox/workflows"),
				Hover("[data-id=heartbeat]"),
				Click("[data-schedule-btn=pause]"),
				Load("/inbox/workflows"),
				Hover("[data-id=heartbeat]"),
			},
			Assert: []Step{
				TextContains("[data-id=heartbeat] .gm-row-labels", "Paused"),
				Visible("[data-schedule-btn=resume]"),
			},
			Name: "pausing a schedule flips the label to paused",
			Path: "/inbox/workflows",
		},
		{
			Act: []Step{
				SeedSchedule(),
				SeedScheduleRun(),
				Load("/inbox/workflows"),
				Click("[data-id=heartbeat] .gm-row-link"),
			},
			Assert: []Step{
				TextContains(".mail-reader-title h1", "heartbeat"),
				TextContains(".mail-reader-labels", "Scheduled"),
				TextContains("#wf-body", "Execution"),
				TextContains("#wf-body", "beat at"),
			},
			Name: "clicking a schedule opens the execution feed",
			Path: "/inbox/workflows",
		},
	})
}
