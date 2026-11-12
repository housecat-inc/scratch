package wire

// ShellCookies names the cookies that hold the app shell's panel
// preferences, which the shell renders as its shell-cookies script for
// ui/dom/shellprefs.ts: the server reads each to draw the next page the way
// the user left it, so the script writes them under the names the server
// reads.
type ShellCookies struct {
	// ChatLast is the chat the chat panel last showed.
	ChatLast string `json:"chat_last"`
	// NavSections is the sidebar sections the user opened or closed.
	NavSections string `json:"nav_sections"`
}

// NavSections is the value of the cookie ShellCookies.NavSections names, as
// JSON: each sidebar section, or label folder, the user opened (true) or
// closed (false) by clicking its header. ui/dom/shellprefs.ts writes it and
// the server reads it, both through this type's Schema.
type NavSections map[string]bool

// ChatLast is the value of the cookie ShellCookies.ChatLast names, as JSON:
// the chat the chat rail last showed. ui/dom/shellprefs.ts writes it and the
// server reads it, both through this type's Schema.
type ChatLast struct {
	ID string `json:"id"`
}
