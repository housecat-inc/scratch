package testkit_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
)

type web struct {
	body string
	code int
	srv  *httptest.Server
}

func visit(path string) tk.Step[*web] {
	return func(t *tk.T, h *web) {
		t.Helper()
		resp, err := http.Get(h.srv.URL + path)
		t.R.NoError(err)
		defer resp.Body.Close()
		b, err := io.ReadAll(resp.Body)
		t.R.NoError(err)
		h.body, h.code = string(b), resp.StatusCode
	}
}

func status(code int) tk.Step[*web] {
	return func(t *tk.T, h *web) { t.Helper(); t.A.Equal(code, h.code) }
}

func contains(text string) tk.Step[*web] {
	return func(t *tk.T, h *web) { t.Helper(); t.A.Contains(h.body, text) }
}

func TestSteps(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/greet", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "hello %s", r.URL.Query().Get("name"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	tk.RunSteps(t, []tk.Scenario[*web]{
		{Name: "greets by name", Steps: []tk.Step[*web]{
			visit("/greet?name=world"),
			status(http.StatusOK),
			contains("hello world"),
		}},
		{Name: "unknown path is not found", Steps: []tk.Step[*web]{
			visit("/missing"),
			status(http.StatusNotFound),
		}},
	}, func(t *tk.T) *web { return &web{srv: srv} })
}
