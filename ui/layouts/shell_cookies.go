package layouts

import (
	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/wire"
)

// ShellCookies are the names of the app shell's preference cookies. The
// server reads them by these names and ShellCookiesScript publishes them to
// ui/dom/shellprefs.ts, which writes them, so a name lives here once.
// The names changed from hc_chat_rail_last and hc_mail_nav when the values
// became JSON, so a value saved in the older form is never read as one.
var ShellCookies = wire.ShellCookies{
	ChatLast:    "hc_shell_chat_last",
	NavSections: "hc_shell_nav_sections",
}

// ShellCookiesID is the id of the script ShellCookiesScript draws.
const ShellCookiesID = "shell-cookies"

// ShellCookiesScript publishes ShellCookies to the page's scripts as JSON.
// A shell draws it once per document; a swap never brings another copy.
func ShellCookiesScript() templ.Component {
	return templ.JSONScript(ShellCookiesID, ShellCookies)
}
