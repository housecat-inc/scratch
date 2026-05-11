package diffview

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeDeps() Deps {
	repos := []repo.Repo{
		{Branch: "feature", Name: "alpha", Org: "acme", Path: "/tmp/alpha",
			LastCommit: git.Commit{SHA: "abc", Subject: "first commit"},
			State:      git.State{Diverged: true, Behind: 2}},
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
	return Deps{
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

func TestOverview(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s, err := NewServer(makeDeps())
	r.NoError(err)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{"acme/alpha", "on feature", "first commit", "2↓"} {
		a.Contains(body, want)
	}
}

func TestDiffPage(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s, err := NewServer(makeDeps())
	r.NoError(err)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/acme/alpha", nil))

	r.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	a.Contains(body, "foo.txt")
	a.Contains(body, "+2")
	a.Contains(body, "−1")
	a.Contains(body, "func main() {")
	a.Contains(body, "direction=up")
	a.Contains(body, "direction=all")
	a.Contains(body, `<span class="font-mono text-xs text-slate-400 truncate flex-1 min-w-0">…</span>`)
}

func TestMissingRepoIs404(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s, err := NewServer(makeDeps())
	r.NoError(err)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/acme/missing", nil))

	a.Equal(http.StatusNotFound, rec.Code)
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
