package sql

import (
	"net/http"
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestSQLHTML(t *testing.T) {
	type step = testkit.Step[*testkit.HTML]

	attrEquals := testkit.AttributeEqualsStep[*testkit.HTML]
	fill := testkit.FillStep[*testkit.HTML]
	textContains := testkit.TextContainsStep[*testkit.HTML]
	visible := testkit.VisibleStep[*testkit.HTML]

	submit := func(selector string) step {
		return func(t *testing.T, h *testkit.HTML) {
			t.Helper()
			h.Submit(selector)
		}
	}

	testkit.RunCases(t, []testkit.Case[*testkit.HTML]{
		{
			Assert: []step{
				attrEquals("#sql-run-form", "hx-post", "/sql/query"),
				visible("#sql-editor"),
				textContains(".tree-pane", "widgets"),
			},
			Name: "renders the editor and schema",
			Path: "/sql/",
		},
		{
			Act: []step{
				fill("#sql-editor", "SELECT name, note FROM widgets ORDER BY id"),
				submit("#sql-run-form"),
			},
			Assert: []step{
				textContains("#sql-results", "alpha"),
				textContains("#sql-results", "bravo"),
				textContains("#sql-results", "hi"),
				textContains("#sql-results", "2 rows"),
			},
			Name: "runs a query and swaps results",
			Path: "/sql/",
		},
		{
			Act: []step{
				fill("#sql-editor", "SELECT * FROM nope"),
				submit("#sql-run-form"),
			},
			Assert: []step{
				textContains("#sql-results", "no such table"),
			},
			Name: "surfaces query errors",
			Path: "/sql/",
		},
	}, testkit.CaseRunner[*testkit.HTML]{
		Load: func(h *testkit.HTML, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.Case[*testkit.HTML]) *testkit.HTML {
			mux := http.NewServeMux()
			mux.Handle("/sql/", http.StripPrefix("/sql", newServer(t, seedDB(t)).Handler()))
			return testkit.NewHTMLWithT(t, kit, mux)
		},
	})
}
