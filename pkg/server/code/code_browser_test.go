package code

import (
	"testing"
	"time"

	"github.com/housecat-inc/scratch/testkit"
)

func TestCodeBrowser(t *testing.T) {
	t.Run("shows the repo overview inside the tool shell", func(t *testing.T) {
		h := codeBrowser(t)
		h.Load("/")
		h.ElementTextContains(".mail-brand", "scratch")
		h.ElementTextContains(".mail-labels", "Code review")
		h.ElementAttributeContains(`a[href="/code/"]`, "class", "active")
		h.ElementTextContains("main", "acme/alpha")
		h.ElementTextContains("main", "first commit")
	})
	t.Run("shows a diff inside the styled tool shell", func(t *testing.T) {
		h := codeBrowser(t)
		h.Load("/acme/alpha")
		h.ElementTextContains(".mail-brand", "scratch")
		h.ElementTextContains("main", "foo.txt")
		h.ElementTextContains("main", "+2")
		shellIconSized(h)
	})
}

func codeBrowser(t *testing.T) *testkit.Harness {
	t.Helper()
	kit := testkit.New(t)
	s, err := NewServer(makeDeps())
	kit.R.NoError(err)
	return testkit.NewHarnessWithT(t, kit, s.Handler())
}

func shellIconSized(h *testkit.Harness) {
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
