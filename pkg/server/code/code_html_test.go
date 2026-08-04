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

	serve := func(t *tk.T, deps Deps) H {
		srv, err := NewServer(deps)
		t.R.NoError(err)
		mux := http.NewServeMux()
		mux.Handle("/code/", http.StripPrefix("/code", srv.Handler()))
		return testkit.NewHTML(t.T, mux)
	}

	s := tk.Steps[H]{}

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
				s.Visit("/code/"),
				s.Text("body", "acme/alpha"),
				s.Text("body", "on feature"),
				s.Text("body", "first commit"),
				s.Text("body", "2↓"),
				s.Present(`a[href="/code/acme/alpha"]`),
				s.Text("body", "Commit & Push"),
				s.Text("body", "Pull"),
				s.AttrContains(`[hx-post="/sessions"]`, "hx-vals", `"dir":"/tmp/alpha"`),
				s.AttrContains(`[hx-post="/sessions"]`, "hx-vals", `"redirect":"1"`),
			},
		},
		{
			Name:  "clean repo has no sync actions",
			Setup: func(t *tk.T) H { return serve(t, cleanRepoDeps(t.T)) },
			Steps: []tk.Step[H]{
				s.Visit("/code/"),
				s.Text("body", "acme/alpha"),
				s.Absent(`[hx-post="/sessions"]`),
			},
		},
		{
			Name: "commits page renders the log",
			Steps: []tk.Step[H]{
				s.Visit("/code/acme/alpha/commits"),
				s.Text("body", "refactor parser"),
				s.Text("body", "first commit"),
				s.Text("body", "abcdef1"),
				s.Present(`a[href="/code/acme/alpha"]`),
				s.Present(`a[href="/code/acme/alpha/commits"]`),
				s.Present(`a[href="/code/acme/alpha/comments"]`),
			},
		},
		{
			Name: "diff page renders hunks and expanders",
			Steps: []tk.Step[H]{
				s.Visit("/code/acme/alpha"),
				s.Text("body", "foo.txt"),
				s.Text("body", "+2"),
				s.Text("body", "−1"),
				s.Text("body", "func main() {"),
				s.Present(`[hx-get*="direction=up"]`),
				s.Present(`[hx-get*="direction=all"]`),
				scriptOnce("/static/chat.js"),
				scriptOnce("/static/htmx-ext-sse.min.js"),
			},
		},
		{
			Name: "missing repo is not found",
			Steps: []tk.Step[H]{
				s.Visit("/code/acme/missing"),
				status(http.StatusNotFound),
			},
		},
		{
			Name:  "adds a comment on the diff",
			Setup: func(t *tk.T) H { return serve(t, commentDeps(t.T)) },
			Steps: []tk.Step[H]{
				s.Visit("/code/acme/alpha"),
				s.Click(addComment),
				s.Fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				s.Submit(commentForm),
				s.Present(`[id^="comment-thread-"] details[id^="comment-"]`),
				s.Text(`[id^="comment-thread-"]`, "nit: rename"),
			},
		},
		{
			Name:  "deletes a comment from the diff",
			Setup: func(t *tk.T) H { return serve(t, commentDeps(t.T)) },
			Steps: []tk.Step[H]{
				s.Visit("/code/acme/alpha"),
				s.Click(addComment),
				s.Fill(commentForm+` textarea[name="body"]`, "nit: rename"),
				s.Submit(commentForm),
				s.Click(`[hx-delete*="/comments/"]`),
				s.Absent(`details[id^="comment-"]`),
			},
		},
	}, func(t *tk.T) H { return serve(t, makeDeps()) })
}
