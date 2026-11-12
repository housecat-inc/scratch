package sql

import (
	"net/http"
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/assert"
)

func TestSQLHTML(t *testing.T) {
	t.Run("editor", func(t *testing.T) {
		t.Run("renders the editor and schema", func(t *testing.T) {
			h := sqlHarness(t)
			h.ElementAttributeEquals("#sql-run-form", "hx-post", "/sql/query")
			h.ElementVisible("#sql-editor")
			h.ElementTextContains(".tree-pane", "widgets")
		})
		t.Run("runs a query and swaps in the results", func(t *testing.T) {
			h := sqlHarness(t)
			h.Fill("#sql-editor", "SELECT name, note FROM widgets ORDER BY id")
			h.Submit("#sql-run-form")
			h.ElementTextContains("#sql-results", "alpha")
			h.ElementTextContains("#sql-results", "bravo")
			h.ElementTextContains("#sql-results", "hi")
			h.ElementTextContains("#sql-results", "2 rows")
		})
		t.Run("surfaces query errors in the results", func(t *testing.T) {
			h := sqlHarness(t)
			h.Fill("#sql-editor", "SELECT * FROM nope")
			h.Submit("#sql-run-form")
			h.ElementTextContains("#sql-results", "no such table")
		})
	})

	t.Run("saved queries", func(t *testing.T) {
		t.Run("saves a query into the sidebar", func(t *testing.T) {
			a := assert.New(t)
			h := sqlHarness(t)
			saveQuery(h, "all widgets", "SELECT * FROM widgets")
			h.ElementTextContains("#sql-query-list", "all widgets")
			a.Equal(1, h.Doc.Find("#sql-query-list li").Length())
		})
		t.Run("upserts one row when a name is reused", func(t *testing.T) {
			a := assert.New(t)
			h := sqlHarness(t)
			saveQuery(h, "all widgets", "SELECT * FROM widgets")
			saveQuery(h, "all widgets", "SELECT id FROM widgets")
			h.ElementTextContains("#sql-query-list", "all widgets")
			a.Equal(1, h.Doc.Find("#sql-query-list li").Length())
		})
		t.Run("deletes a saved query", func(t *testing.T) {
			h := sqlHarness(t)
			saveQuery(h, "all widgets", "SELECT * FROM widgets")
			h.Click(`#sql-query-list [aria-label="Delete query"]`)
			h.ElementAbsent(`#sql-query-list [aria-label="Delete query"]`)
			h.ElementTextContains("#sql-query-list", "none saved")
		})
		t.Run("rejects a save without a name", func(t *testing.T) {
			a := assert.New(t)
			h := sqlHarness(t)
			h.Fill("#sql-editor", "SELECT 1")
			h.Click("#sql-save-confirm")
			a.Equal(http.StatusBadRequest, h.Status)
		})
	})
}

func saveQuery(h *testkit.HTML, name string, sql string) {
	h.Fill("#sql-save-name", name)
	h.Fill("#sql-editor", sql)
	h.Click("#sql-save-confirm")
}

func sqlHarness(t *testing.T) *testkit.HTML {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("/sql/", http.StripPrefix("/sql", newServer(t, seedDB(t)).Handler()))
	h := testkit.NewHTML(t, mux)
	h.Load("/sql/")
	return h
}
