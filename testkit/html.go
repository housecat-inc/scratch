package testkit

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

type HTML struct {
	*T
	Client *http.Client
	Doc    *goquery.Document
	Header http.Header
	Server *httptest.Server
	Status int
}

func NewHTML(t *testing.T, handler http.Handler) *HTML {
	t.Helper()
	return NewHTMLWithT(t, New(t), handler)
}

func NewHTMLWithT(t *testing.T, kit *T, handler http.Handler) *HTML {
	t.Helper()
	server := httptest.NewServer(handler)
	jar, err := cookiejar.New(nil)
	kit.R.NoError(err)
	t.Cleanup(server.Close)
	return &HTML{
		Client: &http.Client{Jar: jar},
		Server: server,
		T:      kit,
	}
}

func (h *HTML) Load(path string) {
	h.t.Helper()
	body := h.do(http.MethodGet, path, nil, false)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	h.R.NoError(err)
	h.Doc = doc
}

func (h *HTML) Click(selector string) {
	h.t.Helper()
	el := h.first(selector)
	if !hasHX(el) {
		if form := el.Closest("form"); form.Length() > 0 && hasHX(form) {
			el = form
		}
	}
	h.dispatch(el)
}

func (h *HTML) Submit(selector string) {
	h.t.Helper()
	h.dispatch(h.first(selector))
}

func (h *HTML) Fill(selector string, text string) {
	h.t.Helper()
	setValue(h.first(selector), text)
}

func (h *HTML) Type(selector string, text string) {
	h.t.Helper()
	el := h.first(selector)
	setValue(el, fieldValue(el)+text)
}

func (h *HTML) SelectOption(selector string, value string) {
	h.t.Helper()
	el := h.first(selector)
	el.Find("option").RemoveAttr("selected")
	el.Find("option[value='"+value+"']").SetAttr("selected", "selected")
}

func (h *HTML) ElementAbsent(selector string) {
	h.t.Helper()
	h.A.Zero(h.find(selector).Length(), "expected %q absent", selector)
}

func (h *HTML) ElementAttributeContains(selector string, name string, expected string) {
	h.t.Helper()
	value, _ := h.first(selector).Attr(name)
	h.A.Contains(value, expected, "attribute %q of %q", name, selector)
}

func (h *HTML) ElementAttributeEquals(selector string, name string, expected string) {
	h.t.Helper()
	value, _ := h.first(selector).Attr(name)
	h.A.Equal(expected, value, "attribute %q of %q", name, selector)
}

func (h *HTML) ElementHidden(selector string) {
	h.t.Helper()
	sel := h.find(selector)
	if sel.Length() == 0 {
		return
	}
	h.A.True(hidden(sel), "expected %q hidden", selector)
}

func (h *HTML) ElementPresent(selector string) {
	h.t.Helper()
	h.A.Positive(h.find(selector).Length(), "expected %q present", selector)
}

func (h *HTML) ElementTextContains(selector string, expected string) {
	h.t.Helper()
	sel := h.find(selector)
	h.R.Positive(sel.Length(), "no element for %q", selector)
	h.A.Contains(sel.Text(), expected)
}

func (h *HTML) ElementVisible(selector string) {
	h.t.Helper()
	sel := h.find(selector)
	h.R.Positive(sel.Length(), "expected %q present", selector)
	h.A.False(hidden(sel), "expected %q visible", selector)
}

func (h *HTML) dispatch(el *goquery.Selection) {
	h.t.Helper()

	method, path, ok := hxRequest(el)
	h.R.True(ok, "element has no hx-get/post/put/patch/delete attribute")

	values := h.collectValues(el)
	target := h.resolveTarget(el)
	swap := attr(el, "hx-swap", "innerHTML")

	var body string
	if method == http.MethodGet {
		if encoded := values.Encode(); encoded != "" {
			if strings.Contains(path, "?") {
				path += "&" + encoded
			} else {
				path += "?" + encoded
			}
		}
		body = h.do(method, path, nil, true)
	} else {
		body = h.do(method, path, strings.NewReader(values.Encode()), true)
	}

	h.applySwap(target, swap, body)
}

func (h *HTML) applySwap(target *goquery.Selection, swap string, body string) {
	switch strings.Fields(swap + " ")[0] {
	case "none":
	case "delete":
		target.Remove()
	case "outerHTML":
		target.ReplaceWithHtml(body)
	default:
		target.SetHtml(body)
	}
}

func (h *HTML) collectValues(el *goquery.Selection) url.Values {
	values := url.Values{}
	if el.Is("form") {
		el.Find("input, select, textarea").Each(func(_ int, node *goquery.Selection) {
			addFieldValue(values, node)
		})
	} else {
		addFieldValue(values, el)
	}
	if include, ok := el.Attr("hx-include"); ok {
		for selector := range strings.SplitSeq(include, ",") {
			h.Doc.Find(strings.TrimSpace(selector)).Each(func(_ int, node *goquery.Selection) {
				addFieldValue(values, node)
			})
		}
	}
	if vals, ok := el.Attr("hx-vals"); ok {
		var parsed map[string]any
		if json.Unmarshal([]byte(vals), &parsed) == nil {
			for key, value := range parsed {
				values.Set(key, fmt.Sprint(value))
			}
		}
	}
	return values
}

func (h *HTML) do(method string, path string, body io.Reader, hx bool) string {
	h.t.Helper()

	contentType := ""
	if body != nil {
		contentType = "application/x-www-form-urlencoded"
	}
	req, err := http.NewRequestWithContext(h.t.Context(), method, h.Server.URL+path, body)
	h.R.NoError(err)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if hx {
		req.Header.Set("HX-Request", "true")
	}

	resp, err := h.Client.Do(req)
	h.R.NoError(err)
	defer resp.Body.Close()

	h.Header = resp.Header
	h.Status = resp.StatusCode
	out, err := io.ReadAll(resp.Body)
	h.R.NoError(err)
	return string(out)
}

func (h *HTML) find(selector string) *goquery.Selection {
	h.t.Helper()
	h.R.NotNil(h.Doc, "no document loaded; call Load first")
	return h.Doc.Find(selector)
}

func (h *HTML) first(selector string) *goquery.Selection {
	h.t.Helper()
	sel := h.find(selector).First()
	h.R.Positive(sel.Length(), "no element for %q", selector)
	return sel
}

func (h *HTML) resolveTarget(el *goquery.Selection) *goquery.Selection {
	target, ok := el.Attr("hx-target")
	switch {
	case !ok, target == "this":
		return el
	case strings.HasPrefix(target, "find "):
		return el.Find(strings.TrimSpace(strings.TrimPrefix(target, "find ")))
	case strings.HasPrefix(target, "closest "):
		return el.Closest(strings.TrimSpace(strings.TrimPrefix(target, "closest ")))
	default:
		return h.Doc.Find(target)
	}
}

var hxVerbs = []struct {
	attr   string
	method string
}{
	{attr: "hx-get", method: http.MethodGet},
	{attr: "hx-post", method: http.MethodPost},
	{attr: "hx-put", method: http.MethodPut},
	{attr: "hx-patch", method: http.MethodPatch},
	{attr: "hx-delete", method: http.MethodDelete},
}

func addFieldValue(values url.Values, node *goquery.Selection) {
	name, ok := node.Attr("name")
	if !ok || name == "" {
		return
	}
	switch goquery.NodeName(node) {
	case "select":
		option := node.Find("option[selected]")
		if option.Length() == 0 {
			option = node.Find("option").First()
		}
		values.Add(name, fieldValue(option))
	case "textarea":
		values.Add(name, node.Text())
	default:
		switch typ, _ := node.Attr("type"); typ {
		case "button", "submit":
			return
		case "checkbox", "radio":
			if _, checked := node.Attr("checked"); !checked {
				return
			}
		}
		value, _ := node.Attr("value")
		values.Add(name, value)
	}
}

func attr(el *goquery.Selection, name string, fallback string) string {
	if value, ok := el.Attr(name); ok {
		return value
	}
	return fallback
}

func fieldValue(el *goquery.Selection) string {
	switch goquery.NodeName(el) {
	case "option", "textarea":
		if value, ok := el.Attr("value"); ok {
			return value
		}
		return el.Text()
	default:
		value, _ := el.Attr("value")
		return value
	}
}

func hasHX(el *goquery.Selection) bool {
	_, _, ok := hxRequest(el)
	return ok
}

func hidden(sel *goquery.Selection) bool {
	if _, ok := sel.Attr("hidden"); ok {
		return true
	}
	style, _ := sel.Attr("style")
	return strings.Contains(strings.ReplaceAll(style, " ", ""), "display:none")
}

func hxRequest(el *goquery.Selection) (string, string, bool) {
	for _, verb := range hxVerbs {
		if path, ok := el.Attr(verb.attr); ok {
			return verb.method, path, true
		}
	}
	return "", "", false
}

func setValue(el *goquery.Selection, value string) {
	if goquery.NodeName(el) == "textarea" {
		el.SetText(value)
		return
	}
	el.SetAttr("value", value)
}
