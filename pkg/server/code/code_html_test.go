package code

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	addCommentButton = `[hx-get*="side=new"][hx-get*="line=4"]`
	commentForm      = `form[hx-post="/code/acme/alpha/comments"]`
)

func TestCodeHTML(t *testing.T) {
	t.Run("overview", func(t *testing.T) {
		t.Run("lists repos with sync actions", func(t *testing.T) {
			h := codeHTML(t, makeDeps())
			h.Load("/code/")
			h.ElementTextContains("body", "acme/alpha")
			h.ElementTextContains("body", "on feature")
			h.ElementTextContains("body", "first commit")
			h.ElementTextContains("body", "2↓")
			h.ElementPresent(`a[href="/code/acme/alpha"]`)
			h.ElementTextContains("body", "Commit & Push")
			h.ElementTextContains("body", "Pull")
			h.ElementAttributeContains(`[hx-post="/sessions"]`, "hx-vals", `"dir":"/tmp/alpha"`)
			h.ElementAttributeContains(`[hx-post="/sessions"]`, "hx-vals", `"redirect":"1"`)
		})
		t.Run("hides sync actions for a clean repo", func(t *testing.T) {
			deps := makeDeps()
			deps.ListRepos = func() ([]repo.Repo, error) {
				return []repo.Repo{{Branch: "feature", Name: "alpha", Org: "acme", Path: "/tmp/alpha"}}, nil
			}
			h := codeHTML(t, deps)
			h.Load("/code/")
			h.ElementTextContains("body", "acme/alpha")
			h.ElementAbsent(`[hx-post="/sessions"]`)
		})
	})

	t.Run("commits page", func(t *testing.T) {
		t.Run("renders the log with repo tabs", func(t *testing.T) {
			h := codeHTML(t, makeDeps())
			h.Load("/code/acme/alpha/commits")
			h.ElementTextContains("body", "refactor parser")
			h.ElementTextContains("body", "first commit")
			h.ElementTextContains("body", "abcdef1")
			h.ElementPresent(`a[href="/code/acme/alpha"]`)
			h.ElementPresent(`a[href="/code/acme/alpha/commits"]`)
			h.ElementPresent(`a[href="/code/acme/alpha/comments"]`)
		})
	})

	t.Run("diff page", func(t *testing.T) {
		t.Run("renders hunks with context expanders", func(t *testing.T) {
			a := assert.New(t)
			h := codeHTML(t, makeDeps())
			h.Load("/code/acme/alpha")
			h.ElementTextContains("body", "foo.txt")
			h.ElementTextContains("body", "+2")
			h.ElementTextContains("body", "−1")
			h.ElementTextContains("body", "func main() {")
			h.ElementPresent(`[hx-get*="direction=up"]`)
			h.ElementPresent(`[hx-get*="direction=all"]`)
			for _, src := range []string{"/static/chat.js", "/static/htmx-ext-sse.min.js"} {
				a.Equal(1, h.Doc.Find(`script[src="`+src+`"]`).Length(), "expected one %s", src)
			}
		})
		t.Run("returns not found for an unknown repo", func(t *testing.T) {
			a := assert.New(t)
			h := codeHTML(t, makeDeps())
			h.Load("/code/acme/missing")
			a.Equal(http.StatusNotFound, h.Status)
		})
	})

	t.Run("diff comments", func(t *testing.T) {
		t.Run("adds a comment thread on a line", func(t *testing.T) {
			h := codeHTML(t, commentDeps(t))
			h.Load("/code/acme/alpha")
			addComment(h, "nit: rename")
			h.ElementPresent(`[id^="comment-thread-"] details[id^="comment-"]`)
			h.ElementTextContains(`[id^="comment-thread-"]`, "nit: rename")
		})
		t.Run("deletes a comment from the thread", func(t *testing.T) {
			h := codeHTML(t, commentDeps(t))
			h.Load("/code/acme/alpha")
			addComment(h, "nit: rename")
			h.Click(`[hx-delete*="/comments/"]`)
			h.ElementAbsent(`details[id^="comment-"]`)
		})
	})
}

func addComment(h *testkit.HTML, body string) {
	h.Click(addCommentButton)
	h.Fill(commentForm+` textarea[name="body"]`, body)
	h.Submit(commentForm)
}

func codeHTML(t *testing.T, deps Deps) *testkit.HTML {
	t.Helper()
	srv, err := NewServer(deps)
	require.New(t).NoError(err)
	mux := http.NewServeMux()
	mux.Handle("/code/", http.StripPrefix("/code", srv.Handler()))
	return testkit.NewHTML(t, mux)
}

func commentDeps(t *testing.T) Deps {
	t.Helper()
	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	require.New(t).NoError(err)
	t.Cleanup(func() { store.Close() })
	deps := makeDeps()
	deps.Comments = store
	return deps
}
