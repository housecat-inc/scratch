package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestInboxTasksBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Act: []Step{
				Type("#chat-input", "buy milk"),
				Click("#chat-send"),
			},
			Assert: []Step{
				TextContains(".mail-reader-title", "buy milk"),
				TextContains(".mail-reader-title", "Active"),
			},
			Name: "creates a task from the footer",
			Path: "/inbox/tasks",
		},
		{
			Assert: []Step{
				TextContains(".mail-list", "active one"),
				ElementAbsent(`[data-id="2"]`),
				ElementAbsent(`[data-id="3"]`),
			},
			Name: "active filter excludes done and archived tasks",
			Path: "/inbox/tasks?archived=active",
			Seed: []Step{
				SeedTask("active one"),
				SeedTask("done one", TaskCompleted(true)),
				SeedTask("archived one", TaskArchived(true)),
			},
		},
		{
			Act: []Step{
				Type(`[name="description"]`, "Call customer before Friday"),
				Click(".mail-save-btn"),
			},
			Assert: []Step{
				TaskDescription(1, "Call customer before Friday"),
				TextContains(".mail-reader-title", "draft"),
			},
			Name: "updates the task description",
			Path: "/inbox/tasks/1",
			Seed: []Step{
				SeedTask("draft"),
			},
		},
	})
}
