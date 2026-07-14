package code

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDeps() Deps {
	repos := []repo.Repo{
		{Branch: "feature", Name: "alpha", Org: "acme", Path: "/tmp/alpha",
			LastCommit: git.Commit{SHA: "abc", Subject: "first commit"},
			State:      git.State{Dirty: true, Diverged: true, Behind: 2}},
	}
	files := []git.File{{
		NewPath: "foo.txt", OldPath: "foo.txt", Status: git.StatusModified,
		Hunks: []git.Hunk{{
			Header:   "@@ -3,3 +3,4 @@ func main() {",
			Section:  "func main() {",
			OldStart: 3, OldCount: 3,
			NewStart: 3, NewCount: 4,
			Lines: []git.Line{
				{Content: "ctx", Kind: git.LineContext, NewLine: 3, OldLine: 3},
				{Content: "old", Kind: git.LineDelete, OldLine: 4},
				{Content: "new", Kind: git.LineAdd, NewLine: 4},
				{Content: "ctx2", Kind: git.LineContext, NewLine: 5, OldLine: 5},
				{Content: "ctx3", Kind: git.LineAdd, NewLine: 6},
			},
		}},
	}}
	contents := []string{"line1", "line2", "ctx", "new", "ctx2", "ctx3", "line7", "line8"}
	commits := []git.Commit{
		{Author: "alice", Date: time.Date(2025, 1, 2, 15, 4, 0, 0, time.UTC), SHA: "abcdef1234567", Subject: "refactor parser"},
		{Author: "bob", Date: time.Date(2025, 1, 1, 12, 0, 0, 0, time.UTC), SHA: "1234567abcdef", Subject: "first commit"},
	}
	return Deps{
		CommitLog: func(r repo.Repo) ([]git.Commit, error) { return commits, nil },
		Diff:      func(r repo.Repo) ([]git.File, error) { return files, nil },
		Home:      "/tmp/home",
		ListRepos: func() ([]repo.Repo, error) { return repos, nil },
		LookupRepo: func(slug string) (repo.Repo, bool) {
			for _, r := range repos {
				if r.Slug() == slug {
					return r, true
				}
			}
			return repo.Repo{}, false
		},
		ShowFile: func(r repo.Repo, path string) ([]string, error) {
			return contents, nil
		},
	}
}

func TestContextEndpoint(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		wantLines []string
	}{
		{
			name:      "up to boundary",
			query:     "path=foo.txt&from=1&to=2&direction=up&bound=1&offset=0",
			wantLines: []string{"line1", "line2"},
		},
		{
			name:      "down with more available",
			query:     "path=foo.txt&from=7&to=7&direction=down&bound=8&offset=-1",
			wantLines: []string{"line7"},
		},
		{
			name:      "down to end",
			query:     "path=foo.txt&from=7&to=8&direction=down&bound=8&offset=-1",
			wantLines: []string{"line7", "line8"},
		},
		{
			name:      "expand all",
			query:     "path=foo.txt&from=1&to=2&direction=all&offset=0",
			wantLines: []string{"line1", "line2"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			s, err := NewServer(makeDeps())
			r.NoError(err)
			rec := httptest.NewRecorder()
			u := &url.URL{Path: "/acme/alpha/file/lines", RawQuery: tc.query}
			s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, u.String(), nil))

			a.Equal(http.StatusOK, rec.Code)
			body := rec.Body.String()
			for _, want := range tc.wantLines {
				a.Contains(body, want)
			}
			a.Contains(body, `data-expand-all="true"`, "buttons always rendered")
		})
	}
}

func TestCommentRoutesValidation(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		body       url.Values
		wantStatus int
	}{
		{
			name:       "missing slug",
			method:     http.MethodPost,
			path:       "/acme/missing/comments",
			body:       url.Values{"path": {"foo.txt"}, "side": {"new"}, "line": {"4"}, "body": {"hi"}},
			wantStatus: http.StatusNotFound,
		},
		{
			name:       "invalid side",
			method:     http.MethodPost,
			path:       "/acme/alpha/comments",
			body:       url.Values{"path": {"foo.txt"}, "side": {"bogus"}, "line": {"4"}, "body": {"hi"}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "empty body",
			method:     http.MethodPost,
			path:       "/acme/alpha/comments",
			body:       url.Values{"path": {"foo.txt"}, "side": {"new"}, "line": {"4"}, "body": {"   "}},
			wantStatus: http.StatusBadRequest,
		},
		{
			name:       "form bad line",
			method:     http.MethodGet,
			path:       "/acme/alpha/comments/form?path=foo.txt&side=new&line=0",
			wantStatus: http.StatusBadRequest,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
			r.NoError(err)
			t.Cleanup(func() { store.Close() })

			deps := makeDeps()
			deps.Comments = store
			s, err := NewServer(deps)
			r.NoError(err)

			var req *http.Request
			if tc.method == http.MethodPost {
				req = httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			} else {
				req = httptest.NewRequest(tc.method, tc.path, nil)
			}
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			require.Equal(t, tc.wantStatus, rec.Code, rec.Body.String())
		})
	}
}

func TestCommentEditFlow(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	deps := makeDeps()
	deps.Comments = store
	s, err := NewServer(deps)
	r.NoError(err)

	c, err := store.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "first")
	r.NoError(err)

	editRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(editRec, httptest.NewRequest(http.MethodGet, "/acme/alpha/comments/"+c.ID+"/edit?view=list", nil))
	r.Equal(http.StatusOK, editRec.Code)
	a.Contains(editRec.Body.String(), `name="body"`)
	a.Contains(editRec.Body.String(), `value="list"`)
	a.Contains(editRec.Body.String(), "first")

	form := url.Values{"body": {"second"}, "view": {"list"}}
	putReq := httptest.NewRequest(http.MethodPut, "/acme/alpha/comments/"+c.ID, strings.NewReader(form.Encode()))
	putReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	putRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(putRec, putReq)
	r.Equal(http.StatusOK, putRec.Code, putRec.Body.String())
	a.Contains(putRec.Body.String(), "second")
	a.NotContains(putRec.Body.String(), `name="body"`)

	got, err := store.GetComment("acme/alpha", c.ID)
	r.NoError(err)
	a.Equal("second", got.Body)

	showRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(showRec, httptest.NewRequest(http.MethodGet, "/acme/alpha/comments/"+c.ID, nil))
	r.Equal(http.StatusOK, showRec.Code)
	a.Contains(showRec.Body.String(), "second")
}

func TestCommentResolveToggle(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	deps := makeDeps()
	deps.Comments = store
	s, err := NewServer(deps)
	r.NoError(err)

	c, err := store.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "ping")
	r.NoError(err)

	post := func() {
		req := httptest.NewRequest(http.MethodPost, "/acme/alpha/comments/"+c.ID+"/resolve", strings.NewReader("view=thread"))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
	}

	post()
	got, err := store.GetComment("acme/alpha", c.ID)
	r.NoError(err)
	a.True(got.Resolved)

	post()
	got, err = store.GetComment("acme/alpha", c.ID)
	r.NoError(err)
	a.False(got.Resolved)
}

func TestCommentListPage(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	deps := makeDeps()
	deps.Comments = store
	s, err := NewServer(deps)
	r.NoError(err)

	_, err = store.AddComment("acme/alpha", "feature", "bar.txt", "new", 2, "bar comment")
	r.NoError(err)
	_, err = store.AddComment("acme/alpha", "feature", "foo.txt", "new", 9, "foo comment")
	r.NoError(err)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/acme/alpha/comments", nil))
	r.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	a.Contains(body, "bar comment")
	a.Contains(body, "foo comment")
	a.Contains(body, "bar.txt")
	a.Contains(body, "foo.txt")
	a.Less(strings.Index(body, "bar.txt"), strings.Index(body, "foo.txt"), "files sorted lexicographically")
}

func TestCommentDeleteReturnsOK(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	deps := makeDeps()
	deps.Comments = store
	s, err := NewServer(deps)
	r.NoError(err)

	c, err := store.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "x")
	r.NoError(err)

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/acme/alpha/comments/"+c.ID, nil))
	a.Equal(http.StatusOK, rec.Code)
	a.Empty(rec.Body.String())

	list, err := store.ListComments("acme/alpha", "feature")
	r.NoError(err)
	a.Empty(list)
}

func TestHandlerSurfacesErrors(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	deps := Deps{
		Home:      "/tmp",
		ListRepos: func() ([]repo.Repo, error) { return nil, errors.New("boom") },
	}
	s, err := NewServer(deps)
	r.NoError(err)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	a.Equal(http.StatusOK, rec.Code)
	a.Contains(rec.Body.String(), "boom")
}
