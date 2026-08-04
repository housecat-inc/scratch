package sql

import (
	"net/http"
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	tk "github.com/housecat-inc/scratch/testkit/v2"
)

type sqlHTML = *testkit.HTML

func sqlHarness(t *tk.T) sqlHTML {
	mux := http.NewServeMux()
	mux.Handle("/sql/", http.StripPrefix("/sql", newServer(t.T, seedDB(t.T)).Handler()))
	return testkit.NewHTML(t.T, mux)
}

func TestSQLHTML(t *testing.T) {
	s := tk.Steps[sqlHTML]{}

	tk.RunSteps(t, []tk.Scenario[sqlHTML]{
		{
			Name: "renders the editor and schema",
			Steps: []tk.Step[sqlHTML]{
				s.Visit("/sql/"),
				s.AttrEquals("#sql-run-form", "hx-post", "/sql/query"),
				s.Visible("#sql-editor"),
				s.Text(".tree-pane", "widgets"),
			},
		},
		{
			Name: "runs a query and swaps results",
			Steps: []tk.Step[sqlHTML]{
				s.Visit("/sql/"),
				s.Fill("#sql-editor", "SELECT name, note FROM widgets ORDER BY id"),
				s.Submit("#sql-run-form"),
				s.Text("#sql-results", "alpha"),
				s.Text("#sql-results", "bravo"),
				s.Text("#sql-results", "hi"),
				s.Text("#sql-results", "2 rows"),
			},
		},
		{
			Name: "surfaces query errors",
			Steps: []tk.Step[sqlHTML]{
				s.Visit("/sql/"),
				s.Fill("#sql-editor", "SELECT * FROM nope"),
				s.Submit("#sql-run-form"),
				s.Text("#sql-results", "no such table"),
			},
		},
	}, sqlHarness)
}

func TestSQLSavedQueriesHTML(t *testing.T) {
	s := tk.Steps[sqlHTML]{}

	save := func(name, sql string) []tk.Step[sqlHTML] {
		return []tk.Step[sqlHTML]{
			s.Fill("#sql-save-name", name),
			s.Fill("#sql-editor", sql),
			s.Click("#sql-save-confirm"),
		}
	}
	rowCount := func(want int) tk.Step[sqlHTML] {
		return func(t *tk.T, h sqlHTML) {
			t.Helper()
			t.A.Equal(want, h.Doc.Find("#sql-query-list li").Length())
		}
	}
	status := func(want int) tk.Step[sqlHTML] {
		return func(t *tk.T, h sqlHTML) { t.Helper(); t.A.Equal(want, h.Status) }
	}

	visit := s.Visit("/sql/")

	tk.RunSteps(t, []tk.Scenario[sqlHTML]{
		{
			Name: "saves a query into the sidebar",
			Steps: append([]tk.Step[sqlHTML]{visit}, append(
				save("all widgets", "SELECT * FROM widgets"),
				s.Text("#sql-query-list", "all widgets"),
				rowCount(1),
			)...),
		},
		{
			Name: "reusing a name upserts one row",
			Steps: append([]tk.Step[sqlHTML]{visit}, append(
				append(save("all widgets", "SELECT * FROM widgets"), save("all widgets", "SELECT id FROM widgets")...),
				s.Text("#sql-query-list", "all widgets"),
				rowCount(1),
			)...),
		},
		{
			Name: "deletes a saved query",
			Steps: append([]tk.Step[sqlHTML]{visit}, append(
				save("all widgets", "SELECT * FROM widgets"),
				s.Click(`#sql-query-list [aria-label="Delete query"]`),
				s.Absent(`#sql-query-list [aria-label="Delete query"]`),
				s.Text("#sql-query-list", "none saved"),
			)...),
		},
		{
			Name: "rejects a save without a name",
			Steps: []tk.Step[sqlHTML]{
				visit,
				s.Fill("#sql-editor", "SELECT 1"),
				s.Click("#sql-save-confirm"),
				status(http.StatusBadRequest),
			},
		},
	}, sqlHarness)
}
