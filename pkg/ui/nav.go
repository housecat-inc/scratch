package ui

import "github.com/housecat-inc/scratch/uikit"

type NavCounts struct {
	Contacts  int
	Inbox     int
	Tasks     int
	Workflows int
}

func appNav(active string, counts NavCounts) []uikit.NavItem {
	return []uikit.NavItem{
		{Active: active == "inbox", Count: counts.Inbox, Href: "/", Icon: "inbox", Label: "Inbox", ShowCount: true},
		{Group: "More", Active: active == "tasks", Count: counts.Tasks, Href: "/tasks", Icon: "task", Label: "Tasks", ShowCount: true},
		{Active: active == "contacts", Count: counts.Contacts, Href: "/contacts", Icon: "contacts", Label: "Contacts", ShowCount: true},
		{Active: active == "workflows", Count: counts.Workflows, Href: "/workflows", Icon: "play", Label: "Workflows", ShowCount: true},
	}
}
