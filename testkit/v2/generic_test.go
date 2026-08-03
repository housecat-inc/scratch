package testkit_test

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	tk "github.com/housecat-inc/scratch/testkit/v2"
)

func TestUpper(t *testing.T) {
	tk.Run(t, []tk.Test[string, string]{
		{In: "one", Out: "ONE"},
		{In: "two", Out: "TWO"},
	}, tk.Pure(strings.ToUpper))
}

func TestAtoi(t *testing.T) {
	tk.Run(t,
		[]tk.Test[string, int]{
			{In: "1", Out: 1},
			{In: "a", Err: "invalid syntax"},
		},
		strconv.Atoi,
	)
}

func TestFixture(t *testing.T) {
	setup := func(t *tk.T) *os.File {
		f, err := os.CreateTemp(t.TempDir(), "buf")
		t.R.NoError(err)
		t.Cleanup(func() { f.Close() })
		return f
	}

	tk.RunF(t,
		[]tk.Test[string, int]{
			{In: "hello", Out: 5},
			{In: "hi", Out: 2},
		},
		setup,
		func(t *tk.T, f *os.File, s string) (int, error) { return f.WriteString(s) },
	)
}

func TestDB(t *testing.T) {
	setup := func(t *tk.T) *sql.DB {
		d, err := sql.Open("sqlite", ":memory:")
		t.R.NoError(err)
		d.SetMaxOpenConns(1)
		_, err = d.Exec("CREATE TABLE names (name TEXT)")
		t.R.NoError(err)
		t.Cleanup(func() { d.Close() })
		return d
	}

	tk.RunF(t,
		[]tk.Test[string, int]{
			{In: "Ada", Out: 1},
			{In: "Bob", Out: 1},
		},
		setup,
		func(t *tk.T, d *sql.DB, name string) (int, error) {
			if _, err := d.Exec("INSERT INTO names (name) VALUES (?)", name); err != nil {
				return 0, err
			}
			var n int
			err := d.QueryRow("SELECT count(*) FROM names").Scan(&n)
			return n, err
		},
		tk.Parallel(),
	)
}

func TestServer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello %s", r.URL.Query().Get("name"))
	}))
	t.Cleanup(srv.Close)

	get := func(query string) (string, error) {
		resp, err := http.Get(srv.URL + query)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		return string(b), err
	}

	tk.Run(t, []tk.Test[string, string]{
		{In: "/?name=world", Out: "hello world"},
		{In: "/?name=go", Out: "hello go"},
	}, get)
}

func TestSplit(t *testing.T) {
	tk.Run(t,
		[]tk.Test[string, []string]{
			{In: "a,b,c", Out: []string{"a", "b", "c"}},
			{In: "x", Out: []string{"x"}},
		},
		func(s string) ([]string, error) { return strings.Split(s, ","), nil },
	)
}
