package sql

import (
	"context"
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	_ "modernc.org/sqlite"
)

const (
	queryMaxRows = 1000
	queryTimeout = 2 * time.Second
)

type Deps struct {
	DefaultPath string
	Store       db.SQLQueryStore
}

func DefaultDeps(home string, store db.SQLQueryStore) Deps {
	return Deps{
		DefaultPath: filepath.Join(home, ".config", "scratch", "scratch.db"),
		Store:       store,
	}
}

type Server struct {
	conns map[string]*stdsql.DB
	deps  Deps
	mu    sync.Mutex
}

func NewServer(deps Deps) (*Server, error) {
	if deps.Store == nil {
		deps.Store = db.NopSQLQueryStore{}
	}
	return &Server{conns: map[string]*stdsql.DB{}, deps: deps}, nil
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", s.handlePage)
	mux.HandleFunc("GET /queries", s.handleListQueries)
	mux.HandleFunc("GET /schema", s.handleSchema)
	mux.HandleFunc("POST /queries", s.handleSaveQuery)
	mux.HandleFunc("POST /query", s.handleQuery)
	mux.HandleFunc("DELETE /queries/{id}", s.handleDeleteQuery)
	return logging(mux)
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.URL.Query().Get("path"))
	vm := ui.SQLProps{Path: path, Query: "SELECT 1;"}
	tables, err := s.tables(r.Context(), path)
	if err != nil {
		vm.Error = err.Error()
	}
	vm.Tables = tables
	if saved, err := s.deps.Store.ListSQLQueries(); err == nil {
		vm.Saved = saved
	}
	s.render(w, r, ui.SQLPage(vm))
}

func (s *Server) handleQuery(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.FormValue("path"))
	query := strings.TrimSpace(r.FormValue("sql"))
	res := &ui.SQLResult{}
	if query == "" {
		res.Error = "empty query"
		s.render(w, r, ui.SQLResults(res))
		return
	}
	if err := s.runQuery(r.Context(), path, query, res); err != nil {
		res.Error = err.Error()
	}
	s.render(w, r, ui.SQLResults(res))
}

func (s *Server) handleSchema(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.URL.Query().Get("path"))
	tables, err := s.tables(r.Context(), path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	out := map[string][]string{}
	for _, t := range tables {
		cols := make([]string, 0, len(t.Columns))
		for _, c := range t.Columns {
			cols = append(cols, c.Name)
		}
		out[t.Name] = cols
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"tables": out})
}

func (s *Server) handleListQueries(w http.ResponseWriter, r *http.Request) {
	saved, err := s.deps.Store.ListSQLQueries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.render(w, r, ui.SQLQueryList(saved))
}

func (s *Server) handleSaveQuery(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimSpace(r.FormValue("name"))
	query := strings.TrimSpace(r.FormValue("sql"))
	if name == "" || query == "" {
		http.Error(w, "name and sql required", http.StatusBadRequest)
		return
	}
	if _, err := s.deps.Store.SaveSQLQuery(name, query); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleListQueries(w, r)
}

func (s *Server) handleDeleteQuery(w http.ResponseWriter, r *http.Request) {
	if err := s.deps.Store.DeleteSQLQuery(r.PathValue("id")); err != nil && !errors.Is(err, db.ErrSQLQueryNotFound) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	s.handleListQueries(w, r)
}

func (s *Server) resolvePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return s.deps.DefaultPath
	}
	return path
}

func (s *Server) conn(path string) (*stdsql.DB, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.conns[path]; ok {
		return c, nil
	}
	if _, err := os.Stat(path); err != nil {
		return nil, errors.Wrapf(err, "open %s", path)
	}
	c, err := stdsql.Open("sqlite", readonlyDSN(path))
	if err != nil {
		return nil, errors.Wrap(err, "open sqlite")
	}
	s.conns[path] = c
	return c, nil
}

func (s *Server) runQuery(ctx context.Context, path, query string, res *ui.SQLResult) error {
	c, err := s.conn(path)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()

	start := time.Now()
	rows, err := c.QueryContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, "query")
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return errors.Wrap(err, "columns")
	}
	res.Columns = cols
	res.Rows = [][]ui.SQLCell{}
	for rows.Next() {
		if len(res.Rows) >= queryMaxRows {
			res.Truncated = true
			break
		}
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return errors.Wrap(err, "scan")
		}
		row := make([]ui.SQLCell, len(cols))
		for i, v := range vals {
			row[i] = formatCell(v)
		}
		res.Rows = append(res.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "rows")
	}
	res.Elapsed = time.Since(start).Round(time.Millisecond).String()
	return nil
}

func (s *Server) tables(ctx context.Context, path string) ([]ui.SQLTable, error) {
	c, err := s.conn(path)
	if err != nil {
		return nil, err
	}
	names, err := c.QueryContext(ctx, `SELECT name FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name ASC`)
	if err != nil {
		return nil, errors.Wrap(err, "list tables")
	}
	defer names.Close()
	var out []ui.SQLTable
	for names.Next() {
		var name string
		if err := names.Scan(&name); err != nil {
			return nil, errors.Wrap(err, "scan table")
		}
		out = append(out, ui.SQLTable{Name: name})
	}
	if err := names.Err(); err != nil {
		return nil, errors.Wrap(err, "tables")
	}
	for i := range out {
		cols, err := s.columns(ctx, c, out[i].Name)
		if err != nil {
			return nil, err
		}
		out[i].Columns = cols
	}
	return out, nil
}

func (s *Server) columns(ctx context.Context, c *stdsql.DB, table string) ([]ui.SQLColumn, error) {
	rows, err := c.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%q)", table))
	if err != nil {
		return nil, errors.Wrapf(err, "table_info %s", table)
	}
	defer rows.Close()
	var out []ui.SQLColumn
	for rows.Next() {
		var (
			cid, notNull, pk int
			dflt             any
			name, typ        string
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			return nil, errors.Wrap(err, "scan column")
		}
		out = append(out, ui.SQLColumn{Name: name, Type: typ})
	}
	return out, errors.Wrap(rows.Err(), "columns")
}

func formatCell(v any) ui.SQLCell {
	switch t := v.(type) {
	case nil:
		return ui.SQLCell{Null: true}
	case []byte:
		return ui.SQLCell{Value: string(t)}
	case time.Time:
		return ui.SQLCell{Value: t.Format(time.RFC3339)}
	case string:
		return ui.SQLCell{Value: t}
	default:
		return ui.SQLCell{Value: fmt.Sprintf("%v", t)}
	}
}

func readonlyDSN(path string) string {
	v := url.Values{}
	v.Set("mode", "ro")
	v.Add("_pragma", "query_only(1)")
	v.Add("_pragma", "busy_timeout(5000)")
	return "file:" + path + "?" + v.Encode()
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, comps ...templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	for _, c := range comps {
		if err := c.Render(r.Context(), w); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.Info("sql request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"remote", r.RemoteAddr,
		)
	})
}
