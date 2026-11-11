// Package formsave is a form that saves in place: it posts through htmx,
// morphs only itself back in from the answer, and says how the save went
// beside its button -- Saving… while it posts, ✓ Saved once it has, or why
// it failed -- with a rejected save drawn back with its field errors and
// what was typed kept. The name says what a caller is choosing: not a
// settings form, but any form whose save comes back to the page it was made
// on.
//
// htmx's own attributes do the posting, swapping, queueing and error
// routing (Form, FormButton, Swap). The browser script ui/dom/formfeedback.ts
// does only what htmx can't express -- the button's ARIA state and
// accessible name, the live region's text, the ✓ that shows for a while,
// and focus on the first invalid field -- and reads each request's outcome
// through ui/dom/request.ts. Submit and StatusLine draw what that script
// moves through; Feedback is what the server draws back into a rejected
// form.
package formsave

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/a-h/templ"
)

// Header is the request header Form, FormButton and Swap tag a save with
// (hx-headers), set to "true". The server reads it to tell a formsave post
// from any other htmx form's -- a sign-in, Update now, a mail thread's note
// -- without guessing from HX-Source or HX-Target, so a failed save is
// answered with a message beside its button.
const Header = "HX-Form-Save"

// StatusKind is how a save went when the server, not the browser, says so:
// an error, or a warning for a save that only partly worked. A plain
// success is a redirect and needs no status.
type StatusKind string

const (
	StatusError   StatusKind = "error"
	StatusWarning StatusKind = "warning"
)

// Status is what a form's status, before its button, says.
type Status struct {
	Kind    StatusKind
	Message string
}

// Feedback is a rejected save, drawn back into the page it came from: the
// form it was (by id), what to say beside its button and under its fields,
// and what was submitted, so nothing typed is lost. The zero value draws
// nothing, which is every page's ordinary GET.
type Feedback struct {
	Fields map[string]string
	Form   string
	Status Status
	Values url.Values
}

// StatusFor is formID's status: the feedback's when it is the rejected form.
func (f Feedback) StatusFor(formID string) Status {
	if formID == "" || f.Form != formID {
		return Status{}
	}
	return f.Status
}

// FieldError is the error under formID's field name.
func (f Feedback) FieldError(formID, name string) string {
	if formID == "" || f.Form != formID {
		return ""
	}
	return f.Fields[name]
}

// Value is what formID's field name shows: what was submitted, when formID
// is the rejected form and name was in it, else stored. A checkbox is drawn
// with Checked, not Value: an unchecked box isn't submitted at all, so its
// absence would fall back to stored here and undo the user's unchecking.
func (f Feedback) Value(formID, name, stored string) string {
	if formID == "" || f.Form != formID {
		return stored
	}
	if v, ok := f.Values[name]; ok && len(v) > 0 {
		return v[0]
	}
	return stored
}

// Checked is whether formID's checkbox name is drawn checked: whether it was
// submitted at all, when formID is the rejected form and its submission is
// kept, since a browser posts a checked box and leaves an unchecked one out;
// else stored.
func (f Feedback) Checked(formID, name string, stored bool) bool {
	if formID == "" || f.Form != formID || f.Values == nil {
		return stored
	}
	_, ok := f.Values[name]
	return ok
}

// Form is a form's attributes, spread on its <form>: it posts to action
// through htmx and morphs only itself back in, picked by id out of whatever
// the save answers. A save that redirects back to the page lands as the
// page's GET, the server's answer for a rejected save is its page with the
// form drawn back (422), and an answer the page can't draw -- echo's JSON
// 4xx or 5xx -- swaps nothing. htmx drops a second submit until the first
// ends (hx-sync); the button stays enabled, so focus stays on it, and reads
// Saving… meanwhile. data-form-feedback hands the outcome to
// ui/dom/formfeedback.ts, which says ✓ Saved or the failure beside the
// button and dispatches settings:saved on the form after a save.
//
// method and action stay so the form still posts without htmx. A form must
// not have a control named "id", which shadows form.id: htmx reads the
// property to match the form in its morph and to name it in HX-Source.
func Form(id, action string, opts ...Opt) templ.Attributes {
	attrs := Swap(id, action, opts...)
	attrs["action"] = action
	attrs["hx-sync"] = "this:drop"
	attrs["hx-target"] = "this"
	attrs["id"] = id
	attrs["method"] = "post"
	return attrs
}

// FormButton is the attributes of a button in the Form form formID that
// posts the form somewhere else, such as a Delete or Remove (Danger): htmx
// posts the form's fields to action from the button and morphs the form
// back in as a save does, with the same opts, and shares the form's queue,
// so a save and a delete never run at once. htmx reads a submitter's
// formaction only for a boosted form, so the button carries its own
// hx-post; formaction and formmethod stay so it still posts to action
// without htmx. They are plain strings: templ drops a templ.SafeURL from an
// Attributes map, which would leave the button posting to the form's own
// action.
func FormButton(formID, action string, opts ...Opt) templ.Attributes {
	attrs := Swap(formID, action, opts...)
	attrs["formaction"] = action
	attrs["formmethod"] = "post"
	attrs["hx-sync"] = "closest form:drop"
	attrs["hx-target"] = "closest form"
	return attrs
}

// Swap is what a Form form and its FormButton buttons post with alike, for
// an element that posts and is not one of them, such as a toggle that
// redraws its own region: to action, morphing the element with id in from
// the answer, and the regions opts name. hx-headers tags the request with
// Header. htmx has no implicit attribute inheritance, so a FormButton button
// carries its own hx-headers rather than read the form's: it is a distinct
// htmx-enhanced element, posting on its own hx-post, not the form's submit.
//
// The 422 rule is not redundant beside 4xx: htmx takes the exact status
// before its class, so without it a rejection would swap nothing.
func Swap(id, action string, opts ...Opt) templ.Attributes {
	attrs := templ.Attributes{
		"data-form-feedback": "",
		"hx-headers":         `{"` + Header + `":"true"}`,
		"hx-post":            action,
		"hx-select":          "#" + cssIdent(id),
		"hx-status:422":      "swap:outerMorph",
		"hx-status:4xx":      "swap:none",
		"hx-status:5xx":      "swap:none",
		"hx-swap":            "outerMorph",
	}
	for _, opt := range opts {
		opt(attrs)
	}
	return attrs
}

// Opt changes what Form, FormButton and Swap return.
type Opt func(templ.Attributes)

// Also morphs the regions with these ids in from the answer along with the
// form, for a save that changes more of the page than its own form, such as
// a mail view's row in its account's table. htmx splits hx-select-oob on a
// plain string split, at each comma and at a selector's first colon, before
// the piece is ever read as a selector (ui/htmx/htmx.min.js #Le:
// `r.split(",")` then `s.split(/:(.*)/)`), so CSS-escaping a comma or a
// colon in id (cssIdent's backslash) does not protect it: the literal
// character is still there to split on. No id with either character can be
// named in hx-select-oob, so Also drops that one region from the morph
// instead of sending a selector that would land on the wrong element. A
// render must not panic over an id it is merely handed, so this checks the
// id itself rather than trust a caller's own validation to have run first.
func Also(ids ...string) Opt {
	return func(attrs templ.Attributes) {
		sels := make([]string, 0, len(ids))
		for _, id := range ids {
			if strings.ContainsAny(id, ",:") {
				continue
			}
			sels = append(sels, "#"+cssIdent(id)+":outerMorph")
		}
		if len(sels) > 0 {
			attrs["hx-select-oob"] = strings.Join(sels, ",")
		}
	}
}

// cssIdent escapes id for a CSS selector, as CSS.escape does for ASCII: a
// character outside [A-Za-z0-9_-] is backslash-escaped, and a leading digit,
// or a digit after a leading hyphen, is written as a code point.
func cssIdent(id string) string {
	var b strings.Builder
	for i := 0; i < len(id); i++ {
		c := id[i]
		switch {
		case c >= '0' && c <= '9' && (i == 0 || i == 1 && id[0] == '-'):
			fmt.Fprintf(&b, `\%x `, c)
		case c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' || c >= 0x80:
			b.WriteByte(c)
		default:
			b.WriteByte('\\')
			b.WriteByte(c)
		}
	}
	return b.String()
}
