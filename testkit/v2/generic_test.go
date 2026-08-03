package testkit_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	testkit "github.com/housecat-inc/scratch/testkit/v2"
)

func TestUpperGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Test[string, string]{
		{In: "one", Out: "ONE"},
		{In: "two", Out: "TWO"},
	}, testkit.Pure(strings.ToUpper))
}

func TestAtoiGeneric(t *testing.T) {
	testkit.Run(t,
		[]testkit.Test[string, int]{
			{In: "1", Out: 1},
			{In: "a", Err: "invalid syntax"},
		},
		strconv.Atoi,
	)
}

func TestFixtureGeneric(t *testing.T) {
	setup := func(t *testkit.T) *os.File {
		f, err := os.CreateTemp(t.TempDir(), "buf")
		t.R.NoError(err)
		t.Cleanup(func() { f.Close() })
		return f
	}

	testkit.RunF(t, setup,
		[]testkit.Test[string, int]{
			{In: "hello", Out: 5},
			{In: "hi", Out: 2},
		},
		func(t *testkit.T, f *os.File, s string) (int, error) { return f.WriteString(s) },
	)
}

func TestDBGeneric(t *testing.T) {
	setup := func(t *testkit.T) *db.DB {
		d, err := db.New(":memory:")
		t.R.NoError(err)
		t.Cleanup(func() { d.Close() })
		return d
	}

	testkit.RunF(t, setup,
		[]testkit.Test[string, int]{
			{In: "Ada", Out: 1},
			{In: "Bob", Out: 1},
		},
		func(t *testkit.T, d *db.DB, name string) (int, error) {
			before, err := d.CountContacts()
			if err != nil {
				return 0, err
			}
			if _, err := d.AddContact(name, "", ""); err != nil {
				return 0, err
			}
			after, err := d.CountContacts()
			if err != nil {
				return 0, err
			}
			return after - before, nil
		},
		testkit.Parallel(),
	)
}

func TestServerGeneric(t *testing.T) {
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

	testkit.Run(t, []testkit.Test[string, string]{
		{In: "/?name=world", Out: "hello world"},
		{In: "/?name=go", Out: "hello go"},
	}, get)
}

func TestSplitGeneric(t *testing.T) {
	testkit.Run(t,
		[]testkit.Test[string, []string]{
			{In: "a,b,c", Out: []string{"a", "b", "c"}},
			{In: "x", Out: []string{"x"}},
		},
		func(s string) ([]string, error) { return strings.Split(s, ","), nil },
	)
}
