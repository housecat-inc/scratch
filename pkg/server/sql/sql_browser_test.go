package sql

import (
	"net/http"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/require"
)

func TestSQLBrowser(t *testing.T) {
	testkit.RunBrowserCases(t, []testkit.BrowserCase[*testkit.Harness]{
		{
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](".mail-brand", "scratch"),
				testkit.TextContainsStep[*testkit.Harness](".mail-labels", "SQL"),
				testkit.ClassContainsStep[*testkit.Harness](`a[href="/sql/"]`, "active"),
				testkit.TextContainsStep[*testkit.Harness](".tree-pane", "widgets"),
			},
			Name: "renders sql browser in tool shell",
			Path: "/sql/",
		},
		{
			Act: []testkit.BrowserStep[*testkit.Harness]{
				testkit.ClickStep[*testkit.Harness]("[data-sql]"),
			},
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness]("#sql-results", "alpha"),
				testkit.TextContainsStep[*testkit.Harness]("#sql-results", "bravo"),
			},
			Name: "runs table query from sidebar",
			Path: "/sql/",
		},
	}, testkit.BrowserCaseRunner[*testkit.Harness]{
		Load: func(h *testkit.Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*testkit.Harness]) *testkit.Harness {
			store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })
			s, err := NewServer(Deps{DefaultPath: seedDB(t), Store: store})
			require.NoError(t, err)
			mux := http.NewServeMux()
			mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			mux.Handle("/sql/", http.StripPrefix("/sql", s.Handler()))
			return testkit.NewHarnessWithT(t, kit, mux)
		},
	})
}
