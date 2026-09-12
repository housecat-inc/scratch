package uikit

//go:generate go tool templ generate

import (
	"context"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type NavItem struct {
	Active   bool
	Child    bool
	Count    int
	CountURL string
	Group    string
	Href     string
	Icon     string
	Label    string
	Section  bool
	Sidebar  bool
}

type ShellProps struct {
	Actions   templ.Component
	AppName   string
	BrandHref string
	Class     string
	Footer    templ.Component
	MainClass string
	Nav       []NavItem
	Sidebar   templ.Component
}

func countLabel(count int) string {
	if count <= 0 {
		return ""
	}
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

func hasSidebarItem(items []NavItem) bool {
	for _, item := range items {
		if item.Sidebar {
			return true
		}
	}
	return false
}

type creationNavigationKey struct{}

func WithCreationNavigation(ctx context.Context, path string) context.Context {
	uri, err := url.ParseRequestURI(path)
	if err != nil {
		return ctx
	}
	creation := ""
	switch uri.Path {
	case "/inbox/chats/new":
		creation = uri.Query().Get("intent")
		if creation != "page" && creation != "workflow" {
			creation = "chat"
		}
	case "/inbox/workflows/new":
		creation = "workflow"
	case "/pages/new":
		creation = "page"
	}
	return context.WithValue(ctx, creationNavigationKey{}, creation)
}

func creationNavigation(ctx context.Context) string {
	creation, _ := ctx.Value(creationNavigationKey{}).(string)
	return creation
}
