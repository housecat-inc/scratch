package ui

import (
	"github.com/housecat-inc/scratch/uikit"
	lucide "github.com/wux4an/lucide-templ/icons"
)

type NavCounts struct {
	Contacts  int
	Inbox     int
	Tasks     int
	Workflows int
}

func appNav(active string, counts NavCounts) []uikit.NavItem {
	return []uikit.NavItem{
		{Active: active == "inbox", Count: counts.Inbox, Href: "/", Icon: lucide.Inbox(), Label: "Inbox", ShowCount: true},
		{Active: active == "tasks", Count: counts.Tasks, Group: "More", Href: "/tasks", Icon: lucide.ListChecks(), Label: "Tasks", ShowCount: true},
		{Active: active == "contacts", Count: counts.Contacts, Href: "/contacts", Icon: lucide.Users(), Label: "Contacts", ShowCount: true},
		{Active: active == "workflows", Count: counts.Workflows, Href: "/workflows", Icon: lucide.Play(), Label: "Workflows", ShowCount: true},
	}
}
