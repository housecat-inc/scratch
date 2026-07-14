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
			},
			Assert: []Step{
				TextContains(".mail-schedules-title", "Schedules"),
				TextContains(".mail-schedule-name", "heartbeat"),
				TextContains(".mail-schedule-status", "Active"),
				Visible("[data-schedule-btn=pause]"),
			},
			Name: "schedules panel lists the heartbeat schedule as active",
			Path: "/inbox/workflows",
		},
		{
			Act: []Step{
				SeedSchedule(),
				Load("/inbox/workflows"),
				Click("[data-schedule-btn=pause]"),
			},
			Assert: []Step{
				TextContains(".mail-schedule-status", "Paused"),
				Visible("[data-schedule-btn=resume]"),
			},
			Name: "pausing a schedule flips the label to paused",
			Path: "/inbox/workflows",
		},
	})
}
