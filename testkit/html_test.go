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
	type step = testkit.Step[*testkit.HTML]

	click := testkit.ClickStep[*testkit.HTML]
	classContains := testkit.ClassContainsStep[*testkit.HTML]
	fill := testkit.FillStep[*testkit.HTML]
	absent := testkit.AbsentStep[*testkit.HTML]
	hidden := testkit.HiddenStep[*testkit.HTML]
	selectOption := testkit.SelectOptionStep[*testkit.HTML]
	textContains := testkit.TextContainsStep[*testkit.HTML]
	typing := testkit.TypeStep[*testkit.HTML]

	testkit.RunCases(t, []testkit.Case[*testkit.HTML]{
		{
			Assert: []step{
				textContains(".brand", "scratch"),
				textContains("#list", "alpha"),
				classContains(".nav", "active"),
				hidden("#secret"),
				textContains("#panel", "original"),
			},
			Name: "renders and queries the page",
			Path: "/",
		},
		{
			Act: []step{
				fill(`[name="q"]`, "hello"),
				click("#form button"),
			},
			Assert: []step{
				textContains("#out", "q=hello"),
				textContains("#out", "color=green"),
			},
			Name: "submit button posts the enclosing form",
			Path: "/",
		},
		{
			Act: []step{
				selectOption(`[name="color"]`, "red"),
				click("#form button"),
			},
			Assert: []step{textContains("#out", "color=red")},
			Name:   "select option changes posted value",
			Path:   "/",
		},
		{
			Act: []step{
				typing(`[name="q"]`, "ab"),
				typing(`[name="q"]`, "cd"),
				click("#form button"),
			},
			Assert: []step{textContains("#out", "q=abcd")},
			Name:   "type appends to the field value",
			Path:   "/",
		},
		{
			Act: []step{
				fill("#inc", "sidecar"),
				click("#save"),
			},
			Assert: []step{textContains("#out", "extra=sidecar")},
			Name:   "hx-include gathers external fields",
			Path:   "/",
		},
		{
			Act: []step{click("#frag")},
			Assert: []step{
				textContains("#panel", "reloaded"),
				absent("#out span"),
			},
			Name: "outerHTML swap replaces the target",
			Path: "/",
		},
		{
			Act:    []step{click("#del")},
			Assert: []step{textContains("#panel", "original")},
			Name:   "none swap leaves the dom unchanged",
			Path:   "/",
		},
	}, testkit.CaseRunner[*testkit.HTML]{
		Load: func(h *testkit.HTML, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.Case[*testkit.HTML]) *testkit.HTML {
			return testkit.NewHTMLWithT(t, kit, htmlHandler())
		},
	})
}
