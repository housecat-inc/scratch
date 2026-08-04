package code

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/testkit"
	tk "github.com/housecat-inc/scratch/testkit/v2"
)

func TestCodePagesHTML(t *testing.T) {
	type H = *testkit.HTML

	newHTML := func(t *tk.T, deps Deps) H {
		s, err := NewServer(deps)
		t.R.NoError(err)
		mux := http.NewServeMux()
		mux.Handle("/code/", http.StripPrefix("/code", s.Handler()))
		return testkit.NewHTML(t.T, mux)
	}
	withDeps := func(build func(*testing.T) Deps) func(*tk.T) H {
		return func(t *tk.T) H { return newHTML(t, build(t.T)) }
	}

	absent := tk.Absent[H]
	attrContains := tk.AttrContains[H]
	click := tk.Click[H]
	fill := tk.Fill[H]
	present := tk.Present[H]
	submit := tk.Submit[H]
	text := tk.Text[H]
	visit := tk.Visit[H]

	status := func(want int) tk.Step[H] {
		return func(t *tk.T, h H) { t.Helper(); t.A.Equal(want, h.Status) }
	}
	scriptOnce := func(src string) tk.Step[H] {
		return func(t *tk.T, h H) {
			t.Helper()
			t.A.Equal(1, h.Doc.Find(`script[src="`+src+`"]`).Length(), "expected one %s", src)
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

	tk.RunSteps(t, []tk.Scenario[H]{
		{
			Name: "overview lists repos with sync actions",
			Steps: []tk.Step[H]{
				visit("/code/"),
				text("body", "acme/alpha"),
				text("body", "on feature"),
				text("body", "first commit"),
				text("body", "2↓"),
				present(`a[href="/code/acme/alpha"]`),
				text("body", "Commit & Push"),
				text("body", "Pull"),
				attrContains(`[hx-post="/sessions"]`, "hx-vals", `"dir":"/tmp/alpha"`),
				attrContains(`[hx-post="/sessions"]`, "hx-vals", `"redirect":"1"`),
			},
		},
		{
			Name:  "clean repo has no sync actions",
			Setup: withDeps(cleanRepoDeps),
			Steps: []tk.Step[H]{
				visit("/code/"),
				text("body", "acme/alpha"),
				absent(`[hx-post="/sessions"]`),
			},
		},
		{
			Name: "commits page renders the log",
			Steps: []tk.Step[H]{
				visit("/code/acme/alpha/commits"),
				text("body", "refactor parser"),
				text("body", "first commit"),
				text("body", "abcdef1"),
				present(`a[href="/code/acme/alpha"]`),
				present(`a[href="/code/acme/alpha/commits"]`),
				present(`a[href="/code/acme/alpha/comments"]`),
			},
		},
		{
			Name: "diff page renders hunks and expanders",
			Steps: []tk.Step[H]{
				visit("/code/acme/alpha"),
				text("body", "foo.txt"),
				text("body", "+2"),
				text("body", "−1"),
				text("body", "func main() {"),
				present(`[hx-get*="direction=up"]`),
				present(`[hx-get*="direction=all"]`),
				scriptOnce("/static/chat.js"),
				scriptOnce("/static/htmx-ext-sse.min.js"),
			},
		},
		{
			Name: "missing repo is not found",
			Steps: []tk.Step[H]{
				visit("/code/acme/missing"),
				status(http.StatusNotFound),
			},
		},
		{
			Name:  "adds a comment on the diff",
			Setup: withDeps(commentDeps),
			Steps: []tk.Step[H]{
				visit("/code/acme/alpha"),
				click(addComment),
				fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				submit(commentForm),
				present(`[id^="comment-thread-"] details[id^="comment-"]`),
				text(`[id^="comment-thread-"]`, "nit: rename"),
			},
		},
		{
			Name:  "deletes a comment from the diff",
			Setup: withDeps(commentDeps),
			Steps: []tk.Step[H]{
				visit("/code/acme/alpha"),
				click(addComment),
				fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				submit(commentForm),
				click(`[hx-delete*="/comments/"]`),
				absent(`details[id^="comment-"]`),
			},
		},
	}, func(t *tk.T) H { return newHTML(t, makeDeps()) })
}
