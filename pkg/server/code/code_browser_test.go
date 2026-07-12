package code

import (
	"testing"
	"time"

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
		{
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](".mail-brand", "scratch"),
				testkit.TextContainsStep[*testkit.Harness]("main", "foo.txt"),
				testkit.TextContainsStep[*testkit.Harness]("main", "+2"),
				shellIconSizedStep(),
			},
			Name: "renders diff in styled tool shell",
			Path: "/acme/alpha",
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

func shellIconSizedStep() testkit.BrowserStep[*testkit.Harness] {
	return func(t *testing.T, h *testkit.Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			result, err := h.Page.Eval(`() => {
				const shell = document.querySelector(".mail-shell");
				const svg = document.querySelector(".gm-label.active .gm-label-icon svg");
				if (!shell || !svg) return false;
				const box = svg.getBoundingClientRect();
				return getComputedStyle(shell).display === "grid" && box.width > 0 && box.width <= 32 && box.height > 0 && box.height <= 32;
			}`)
			return err == nil && result.Value.Bool()
		}, 5*time.Second, 50*time.Millisecond)
	}
}
