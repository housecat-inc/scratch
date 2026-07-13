package sessions

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestSetupBrowser(t *testing.T) {
	testkit.RunBrowserCases(t, []testkit.BrowserCase[*testkit.Harness]{
		{
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](".mail-brand", "scratch"),
				testkit.TextContainsStep[*testkit.Harness](".mail-labels", "Setup"),
				testkit.TextContainsStep[*testkit.Harness](".mail-labels", "Code review"),
				testkit.ClassContainsStep[*testkit.Harness](`a[href="/setup"]`, "active"),
				testkit.TextContainsStep[*testkit.Harness]("#card-install", "Install"),
				testkit.TextContainsStep[*testkit.Harness]("#card-login", "Sign in"),
				testkit.TextContainsStep[*testkit.Harness]("#card-configure", "Configure"),
			},
			Name: "renders setup in tool shell",
			Path: "/setup",
		},
	}, testkit.BrowserCaseRunner[*testkit.Harness]{
		Load: func(h *testkit.Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*testkit.Harness]) *testkit.Harness {
			fake := fakeDeps{}
			s, err := NewServer(fake.deps())
			kit.R.NoError(err)
			return testkit.NewHarnessWithT(t, kit, s.Handler())
		},
	})
}
