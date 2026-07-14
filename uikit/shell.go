package uikit

//go:generate go tool templ generate

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type NavItem struct {
	Active    bool
	Count     int
	Group     string
	Href      string
	Icon      string
	Label     string
	ShowCount bool
}

type ShellProps struct {
	Actions    templ.Component
	AppName    string
	BrandHref  string
	Class      string
	Footer     templ.Component
	MainClass  string
	Nav        []NavItem
	SidebarTop templ.Component
}

func countLabel(count int) string {
	return strconv.Itoa(count)
}

func historyHref(href string) string {
	if href == "" {
		return "/chats"
	}
	return href
}

func mainClass(extra string, footer templ.Component) string {
	classes := []string{"mail-main"}
	if footer == nil {
		classes = append(classes, "mail-main-no-footer")
	}
	if extra != "" {
		classes = append(classes, extra)
	}
	return strings.Join(classes, " ")
}

func shellClass(extra string) string {
	if extra == "" {
		return "mail-shell"
	}
	return "mail-shell " + extra
}

func newChatAction(action string) string {
	if action == "" {
		return "/chat/popout/new"
	}
	return action
}
