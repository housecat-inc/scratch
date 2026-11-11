package wire

// NavMove is one of the moves a mail settings table posts to
// /settings/mail/nav with op=order (its moves field, a list): the entry
// moved, the section it lands in, and the entry it lands before, empty for
// the section's end.
type NavMove struct {
	Before  string `json:"before"`
	Key     string `json:"key"`
	Section string `json:"section"`
}

// NavVisibilityChange is one of the sidebar visibilities a mail settings
// table posts to /settings/mail/nav with op=visibility (its changes field, a
// list): the mailbox, the entry, and whether the sidebar shows it.
type NavVisibilityChange struct {
	Account string `json:"account"`
	Key     string `json:"key"`
	Visible bool   `json:"visible"`
}
