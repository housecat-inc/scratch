package code

import (
	"net/http"
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestCodePagesHTML(t *testing.T) {
	type step = testkit.Step[*testkit.HTML]

	attrContains := testkit.AttributeContainsStep[*testkit.HTML]
	present := testkit.PresentStep[*testkit.HTML]
	textContains := testkit.TextContainsStep[*testkit.HTML]

	status := func(want int) step {
		return func(t *testing.T, h *testkit.HTML) {
			t.Helper()
			h.A.Equal(want, h.Status)
		}
	}
	scriptOnce := func(src string) step {
		return func(t *testing.T, h *testkit.HTML) {
			t.Helper()
			h.A.Equal(1, h.Doc.Find(`script[src="`+src+`"]`).Length(), "expected one %s", src)
		}
	}

	testkit.RunCases(t, []testkit.Case[*testkit.HTML]{
		{
			Assert: []step{
				textContains("body", "acme/alpha"),
				textContains("body", "on feature"),
				textContains("body", "first commit"),
				textContains("body", "2↓"),
				present(`a[href="/code/acme/alpha"]`),
				textContains("body", "Commit & Push"),
				textContains("body", "Pull"),
				attrContains(`[hx-post="/sessions"]`, "hx-vals", `"dir":"/tmp/alpha"`),
				attrContains(`[hx-post="/sessions"]`, "hx-vals", `"redirect":"1"`),
			},
			Name: "overview lists repos with sync actions",
			Path: "/code/",
		},
		{
			Assert: []step{
				textContains("body", "refactor parser"),
				textContains("body", "first commit"),
				textContains("body", "abcdef1"),
				present(`a[href="/code/acme/alpha"]`),
				present(`a[href="/code/acme/alpha/commits"]`),
				present(`a[href="/code/acme/alpha/comments"]`),
			},
			Name: "commits page renders the log",
			Path: "/code/acme/alpha/commits",
		},
		{
			Assert: []step{
				textContains("body", "foo.txt"),
				textContains("body", "+2"),
				textContains("body", "−1"),
				textContains("body", "func main() {"),
				present(`[hx-get*="direction=up"]`),
				present(`[hx-get*="direction=all"]`),
				scriptOnce("/static/chat.js"),
				scriptOnce("/static/htmx-ext-sse.min.js"),
			},
			Name: "diff page renders hunks and expanders",
			Path: "/code/acme/alpha",
		},
		{
			Assert: []step{status(http.StatusNotFound)},
			Name:   "missing repo is not found",
			Path:   "/code/acme/missing",
		},
	}, testkit.CaseRunner[*testkit.HTML]{
		Load: func(h *testkit.HTML, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.Case[*testkit.HTML]) *testkit.HTML {
			s, err := NewServer(makeDeps())
			kit.R.NoError(err)
			mux := http.NewServeMux()
			mux.Handle("/code/", http.StripPrefix("/code", s.Handler()))
			return testkit.NewHTMLWithT(t, kit, mux)
		},
	})
}
