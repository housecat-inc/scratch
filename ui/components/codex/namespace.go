package codex

import "net/url"

// Namespace is the one place the product name behind this chat is written.
// It is the tool's name today and will not always be; URL paths and DOM
// event names are both built from it so renaming is one edit and no string
// in the codebase has to be hunted for.
const Namespace = "codex"

// ThreadPlaceholder stands in for a thread id inside a URL template handed
// to the browser before that thread exists. outgoing.templ's connect()
// learns a new thread's real id only once the server answers the first
// optimistic send, so a surface can only hand the script a pattern up
// front; connect() substitutes the real id into that pattern rather than
// rebuilding the URL itself. It is letters and underscores only, so neither
// url.PathEscape nor url.Values.Encode -- used wherever a surface builds one
// of these templates -- alters it, and the same literal survives for the
// script to find and replace.
const ThreadPlaceholder = "__thread__"

// Path is the URL prefix every chat route and link sits under.
const Path = "/" + Namespace

// MailContentID is the id of the chat panel in the mail rail: the element a
// rail open swaps by outerHTML and the source of that request, so the panel
// with this id is the one whose fetches replace each other.
const MailContentID = "mail-chat-rail-content"

// StopPath is where a running turn is stopped. It is empty without a turn,
// since a stop names the turn it is meant for and the server refuses one for
// any other.
func StopPath(threadID string, turnID string) string {
	if threadID == "" || turnID == "" {
		return ""
	}
	return Path + "/threads/" + url.PathEscape(threadID) + "/turns/" + url.PathEscape(turnID) + "/stop"
}
