package diffview

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler(t *testing.T) {
	repos := []repo.Repo{
		{Branch: "feature", Name: "alpha", Org: "acme", Path: "/tmp/alpha",
			LastCommit: git.Commit{SHA: "abc", Subject: "first commit"},
			State:      git.State{Diverged: true, Behind: 2}},
		{Branch: "main", Name: "beta", Org: "acme", Path: "/tmp/beta",
			LastCommit: git.Commit{SHA: "def", Subject: "second commit"}},
	}
	files := []git.File{{
		NewPath: "foo.txt", OldPath: "foo.txt", Status: git.StatusModified,
		Hunks: []git.Hunk{{
			Header:   "@@ -1,2 +1,3 @@",
			OldStart: 1, OldCount: 2,
			NewStart: 1, NewCount: 3,
			Lines: []git.Line{
				{Content: "ctx", Kind: git.LineContext, NewLine: 1, OldLine: 1},
				{Content: "old", Kind: git.LineDelete, OldLine: 2},
				{Content: "new", Kind: git.LineAdd, NewLine: 2},
			},
		}},
	}}

	deps := Deps{
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
	}

	tests := []struct {
		name    string
		path    string
		status  int
		bodyHas []string
	}{
		{
			name:    "overview lists repos",
			path:    "/",
			status:  http.StatusOK,
			bodyHas: []string{"acme/alpha", "acme/beta", "on feature", "first commit", "2↓"},
		},
		{
			name:    "diff shows hunks",
			path:    "/acme/alpha",
			status:  http.StatusOK,
			bodyHas: []string{"foo.txt", "@@ -1,2 &#43;1,3 @@", "old", "new", "ctx"},
		},
		{
			name:   "missing repo is 404",
			path:   "/acme/missing",
			status: http.StatusNotFound,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			s, err := NewServer(deps)
			r.NoError(err)
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tc.path, nil))

			a.Equal(tc.status, rec.Code)
			for _, want := range tc.bodyHas {
				a.Contains(rec.Body.String(), want)
			}
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
