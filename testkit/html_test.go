package testkit_test

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func htmlHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<!doctype html><html><body>
			<span class="brand">scratch</span>
			<ul id="list"><li data-id="1">alpha</li><li data-id="2">bravo</li></ul>
			<a class="nav active" href="/">Home</a>
			<div id="secret" hidden>hidden text</div>
			<form id="form" hx-post="/echo" hx-target="#out" hx-swap="innerHTML">
				<input name="q" value=""/>
				<textarea name="note"></textarea>
				<select name="color">
					<option value="red">red</option>
					<option value="green" selected>green</option>
				</select>
				<button type="submit">Run</button>
			</form>
			<input id="inc" name="extra" value=""/>
			<button id="save" hx-post="/save" hx-target="#out" hx-swap="innerHTML" hx-include="#inc">Save</button>
			<a id="frag" hx-get="/frag" hx-target="#panel" hx-swap="outerHTML">reload</a>
			<button id="del" hx-delete="/item" hx-target="#panel" hx-swap="none">delete</button>
			<div id="panel">original</div>
			<div id="out"></div>
		</body></html>`)
	})

	mux.HandleFunc("POST /echo", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		fmt.Fprint(w, formSummary(r))
	})
	mux.HandleFunc("POST /save", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		fmt.Fprint(w, formSummary(r))
	})
	mux.HandleFunc("GET /frag", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `<div id="panel">reloaded</div>`)
	})
	mux.HandleFunc("DELETE /item", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	return mux
}

func formSummary(r *http.Request) string {
	pairs := make([]string, 0, len(r.Form))
	for key, values := range r.Form {
		pairs = append(pairs, key+"="+strings.Join(values, ","))
	}
	sort.Strings(pairs)
	return "<span id=\"echoed\">" + strings.Join(pairs, " ") + "</span>"
}

func TestHTML(t *testing.T) {
	load := func(t *testing.T) *testkit.HTML {
		h := testkit.NewHTML(t, htmlHandler())
		h.Load("/")
		return h
	}

	t.Run("Load", func(t *testing.T) {
		t.Run("renders and queries the page", func(t *testing.T) {
			h := load(t)
			h.ElementTextContains(".brand", "scratch")
			h.ElementTextContains("#list", "alpha")
			h.ElementAttributeContains(".nav", "class", "active")
			h.ElementHidden("#secret")
			h.ElementTextContains("#panel", "original")
		})
	})

	t.Run("Click", func(t *testing.T) {
		t.Run("submit button posts the enclosing form", func(t *testing.T) {
			h := load(t)
			h.Fill(`[name="q"]`, "hello")
			h.Click("#form button")
			h.ElementTextContains("#out", "q=hello")
			h.ElementTextContains("#out", "color=green")
		})
		t.Run("hx-include gathers external fields", func(t *testing.T) {
			h := load(t)
			h.Fill("#inc", "sidecar")
			h.Click("#save")
			h.ElementTextContains("#out", "extra=sidecar")
		})
		t.Run("outerHTML swap replaces the target", func(t *testing.T) {
			h := load(t)
			h.Click("#frag")
			h.ElementTextContains("#panel", "reloaded")
			h.ElementAbsent("#out span")
		})
		t.Run("none swap leaves the dom unchanged", func(t *testing.T) {
			h := load(t)
			h.Click("#del")
			h.ElementTextContains("#panel", "original")
		})
	})

	t.Run("SelectOption", func(t *testing.T) {
		t.Run("changes the posted value", func(t *testing.T) {
			h := load(t)
			h.SelectOption(`[name="color"]`, "red")
			h.Click("#form button")
			h.ElementTextContains("#out", "color=red")
		})
	})

	t.Run("Type", func(t *testing.T) {
		t.Run("appends to the field value", func(t *testing.T) {
			h := load(t)
			h.Type(`[name="q"]`, "ab")
			h.Type(`[name="q"]`, "cd")
			h.Click("#form button")
			h.ElementTextContains("#out", "q=abcd")
		})
	})
}
