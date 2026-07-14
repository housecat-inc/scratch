package code

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/testkit"
)

func TestCodePagesHTML(t *testing.T) {
	type step = testkit.Step[*testkit.HTML]

	absent := testkit.AbsentStep[*testkit.HTML]
	attrContains := testkit.AttributeContainsStep[*testkit.HTML]
	click := testkit.ClickStep[*testkit.HTML]
	fill := testkit.FillStep[*testkit.HTML]
	present := testkit.PresentStep[*testkit.HTML]
	textContains := testkit.TextContainsStep[*testkit.HTML]

	submit := func(selector string) step {
		return func(t *testing.T, h *testkit.HTML) {
			t.Helper()
			h.Submit(selector)
		}
	}
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

	cleanRepoDeps := func(*testing.T) Deps {
		deps := makeDeps()
		deps.ListRepos = func() ([]repo.Repo, error) {
			return []repo.Repo{{Branch: "feature", Name: "alpha", Org: "acme", Path: "/tmp/alpha"}}, nil
		}
		return deps
	}
	commentDeps := func(t *testing.T) Deps {
		store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
		testkit.New(t).R.NoError(err)
		t.Cleanup(func() { store.Close() })
		deps := makeDeps()
		deps.Comments = store
		return deps
	}

	addComment := `[hx-get*="side=new"][hx-get*="line=4"]`
	commentForm := `form[hx-post="/code/acme/alpha/comments"]`

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
				textContains("body", "acme/alpha"),
				absent(`[hx-post="/sessions"]`),
			},
			Data: cleanRepoDeps,
			Name: "clean repo has no sync actions",
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
		{
			Act: []step{
				click(addComment),
				fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				submit(commentForm),
			},
			Assert: []step{
				present(`[id^="comment-thread-"] details[id^="comment-"]`),
				textContains(`[id^="comment-thread-"]`, "nit: rename"),
			},
			Data: commentDeps,
			Name: "adds a comment on the diff",
			Path: "/code/acme/alpha",
		},
		{
			Act: []step{
				click(addComment),
				fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				submit(commentForm),
				click(`[hx-delete*="/comments/"]`),
			},
			Assert: []step{
				absent(`details[id^="comment-"]`),
			},
			Data: commentDeps,
			Name: "deletes a comment from the diff",
			Path: "/code/acme/alpha",
		},
	}, testkit.CaseRunner[*testkit.HTML]{
		Load: func(h *testkit.HTML, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, tc testkit.Case[*testkit.HTML]) *testkit.HTML {
			deps := makeDeps()
			if tc.Data != nil {
				deps = tc.Data.(func(*testing.T) Deps)(t)
			}
			s, err := NewServer(deps)
			kit.R.NoError(err)
			mux := http.NewServeMux()
			mux.Handle("/code/", http.StripPrefix("/code", s.Handler()))
			return testkit.NewHTMLWithT(t, kit, mux)
		},
	})
}
