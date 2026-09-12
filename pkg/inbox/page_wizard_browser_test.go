package inbox

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestPageWizardBrowser(t *testing.T) {
	runBrowser(t, []testkit.BrowserCase[*Harness]{
		{
			Name: "page brief can be reviewed edited and sent to chat",
			Path: "/pages",
			Act: []Step{
				Click(`a[data-new-item="page"]`),
				Click(`#wizard-next`),
				Visible(`#page-title`),
				Type(`#page-title`, "Team dashboard"),
				Type(`#page-description`, "Track our projects"),
				Click(`#wizard-next`),
				Type(`#page-details`, "Group by status"),
				Click(`#wizard-next`),
				TextContains(`#page-review`, "Group by status"),
				Click(`#wizard-back`),
				Visible(`#page-details`),
				Click(`#wizard-next`),
				Click(`#wizard-submit`),
			},
			Assert: []Step{TextContains(`body`, "Page name: Team dashboard"), TextContains(`body`, "Group by status")},
		},
		{
			Name:   "cancel returns to pages",
			Path:   "/pages/new",
			Act:    []Step{Click(`.workflow-wizard-cancel`)},
			Assert: []Step{Visible(`.pages-grid`)},
		},
	})
}
