package sql

import (
	stdsql "database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func seedDB(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "data.db")
	conn, err := stdsql.Open("sqlite", "file:"+path+"?_pragma=journal_mode(WAL)")
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Exec(`CREATE TABLE widgets (id INTEGER PRIMARY KEY, name TEXT, note TEXT);
		INSERT INTO widgets (id, name, note) VALUES (1, 'alpha', NULL), (2, 'bravo', 'hi');`)
	require.NoError(t, err)
	return path
}

func newServer(t *testing.T, path string) *Server {
	t.Helper()
	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	s, err := NewServer(Deps{DefaultPath: path, Store: store})
	require.NoError(t, err)
	return s
}

func TestPage(t *testing.T) {
	a := assert.New(t)
	s := newServer(t, seedDB(t))

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	for _, want := range []string{
		"scratch", "widgets", "SQL",
		`class="gm-label active"`,
		`hx-post="/sql/query"`,
		`id="sql-editor"`,
		`/static/sql.js`,
	} {
		a.Contains(body, want)
	}
}

func TestQuery(t *testing.T) {
	a := assert.New(t)
	s := newServer(t, seedDB(t))

	form := url.Values{"sql": {"SELECT name, note FROM widgets ORDER BY id"}}
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	body := rec.Body.String()
	a.Contains(body, "alpha")
	a.Contains(body, "bravo")
	a.Contains(body, "hi")
	a.Contains(body, "NULL")
	a.Contains(body, "2 rows")
}

func TestQueryError(t *testing.T) {
	a := assert.New(t)
	s := newServer(t, seedDB(t))

	form := url.Values{"sql": {"SELECT * FROM nope"}}
	req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	a.Equal(http.StatusOK, rec.Code)
	a.Contains(rec.Body.String(), "no such table")
}

func TestQueryReadOnly(t *testing.T) {
	tests := []struct {
		name string
		sql  string
	}{
		{name: "insert", sql: "INSERT INTO widgets (id, name) VALUES (3, 'charlie')"},
		{name: "delete", sql: "DELETE FROM widgets"},
		{name: "drop", sql: "DROP TABLE widgets"},
		{name: "pragma bypass", sql: "PRAGMA query_only=0; INSERT INTO widgets (id, name) VALUES (3, 'x')"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			path := seedDB(t)
			s := newServer(t, path)

			form := url.Values{"sql": {tc.sql}}
			req := httptest.NewRequest(http.MethodPost, "/query", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			rec := httptest.NewRecorder()
			s.Handler().ServeHTTP(rec, req)
			a.Equal(http.StatusOK, rec.Code)

			conn, err := stdsql.Open("sqlite", "file:"+path)
			r.NoError(err)
			defer conn.Close()
			var count int
			r.NoError(conn.QueryRow("SELECT COUNT(*) FROM widgets").Scan(&count))
			a.Equal(2, count, "row count unchanged after %q", tc.sql)
		})
	}
}

func TestSchema(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	s := newServer(t, seedDB(t))

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/schema", nil))
	r.Equal(http.StatusOK, rec.Code)

	var out struct {
		Tables map[string][]string `json:"tables"`
	}
	r.NoError(json.Unmarshal(rec.Body.Bytes(), &out))
	a.Equal([]string{"id", "name", "note"}, out.Tables["widgets"])
}

func TestSavedQueries(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	s := newServer(t, seedDB(t))

	save := func(name, sql string) {
		form := url.Values{"name": {name}, "sql": {sql}}
		req := httptest.NewRequest(http.MethodPost, "/queries", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		r.Equal(http.StatusOK, rec.Code, rec.Body.String())
	}

	save("all widgets", "SELECT * FROM widgets")
	list, err := s.deps.Store.ListSQLQueries()
	r.NoError(err)
	r.Len(list, 1)
	a.Contains(list[0].SQL, "SELECT * FROM widgets")

	save("all widgets", "SELECT id FROM widgets")
	list, err = s.deps.Store.ListSQLQueries()
	r.NoError(err)
	r.Len(list, 1, "same name upserts")
	a.Equal("SELECT id FROM widgets", list[0].SQL)

	listRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/queries", nil))
	a.Contains(listRec.Body.String(), "all widgets")

	delRec := httptest.NewRecorder()
	s.Handler().ServeHTTP(delRec, httptest.NewRequest(http.MethodDelete, "/queries/"+list[0].ID, nil))
	a.Equal(http.StatusOK, delRec.Code)
	list, err = s.deps.Store.ListSQLQueries()
	r.NoError(err)
	a.Empty(list)
}

func TestSaveValidation(t *testing.T) {
	a := assert.New(t)
	s := newServer(t, seedDB(t))

	form := url.Values{"name": {""}, "sql": {"SELECT 1"}}
	req := httptest.NewRequest(http.MethodPost, "/queries", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	a.Equal(http.StatusBadRequest, rec.Code)
}

func TestMissingDBSurfacesError(t *testing.T) {
	a := assert.New(t)
	s := newServer(t, filepath.Join(t.TempDir(), "absent.db"))

	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	a.Equal(http.StatusOK, rec.Code)
	a.Contains(rec.Body.String(), "absent.db")
}
