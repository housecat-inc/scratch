package code

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestCodeBrowser(t *testing.T) {
	testkit.RunBrowserCases(t, []testkit.BrowserCase[*testkit.Harness]{
		{
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](".mail-brand", "scratch"),
				testkit.TextContainsStep[*testkit.Harness](".mail-labels", "Code review"),
				testkit.ClassContainsStep[*testkit.Harness](`a[href="/code/"]`, "active"),
				testkit.TextContainsStep[*testkit.Harness]("main", "acme/alpha"),
				testkit.TextContainsStep[*testkit.Harness]("main", "first commit"),
			},
			Name: "renders overview in tool shell",
			Path: "/",
		},
	}, testkit.BrowserCaseRunner[*testkit.Harness]{
		Load: func(h *testkit.Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*testkit.Harness]) *testkit.Harness {
			s, err := NewServer(makeDeps())
			kit.R.NoError(err)
			return testkit.NewHarnessWithT(t, kit, s.Handler())
		},
	})
}
