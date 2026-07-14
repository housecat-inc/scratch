package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func SeedSchedule() Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.NoError(h.Flows.EnsureSchedules())
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
				TextContains("[data-id=heartbeat] .gm-row-what", "heartbeat"),
				TextContains("[data-id=heartbeat] .gm-row-date", "Active"),
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
				TextContains("[data-id=heartbeat] .gm-row-date", "Paused"),
				Visible("[data-schedule-btn=resume]"),
			},
			Name: "pausing a schedule flips the label to paused",
			Path: "/inbox/workflows",
		},
	})
}
