package uikit

//go:generate go tool templ generate

import (
	"context"
	"strconv"

	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/app"
	"github.com/housecat-inc/scratch/ui/icons"
)

type NavItem struct {
	Active    bool
	Count     int
	Group     string
	Href      string
	Icon      templ.Component
	IconName  icons.Name
	Label     string
	ShowCount bool
}

type ShellProps struct {
	Actions    templ.Component
	AppName    string
	BrandHref  string
	Class      string
	MainClass  string
	Nav        []NavItem
	SidebarTop templ.Component
}

func shellIn(p ShellProps) app.ShellIn {
	nav := make([]app.NavItem, 0, len(p.Nav))
	for _, item := range p.Nav {
		count := ""
		if item.ShowCount && item.Count > 0 {
			count = strconv.Itoa(item.Count)
		}
		nav = append(nav, app.NavItem{Count: count, Current: item.Active, Group: item.Group, Href: item.Href, Icon: item.IconName, IconComponent: iconComponent(item), Label: item.Label})
	}
	return app.ShellIn{
		BrandHref: p.BrandHref,
		BrandName: p.AppName,
		Chat:      chatRailContent(),
		Nav:       nav,
		NavHeader: p.SidebarTop,
		TopBar:    p.Actions,
	}
}

func mainClass(extra string) string {
	if extra == "" {
		return "flex min-h-0 flex-1 flex-col overflow-y-auto"
	}
	return "flex min-h-0 flex-1 flex-col overflow-y-auto " + extra
}

func historyHref(href string) string {
	if href == "" {
		return "/chats"
	}
	return href
}

func countLabel(count int) string {
	return strconv.Itoa(count)
}

func iconComponent(item NavItem) templ.Component {
	if item.IconName != "" {
		return nil
	}
	return item.Icon
}

func withRail(ctx context.Context, in app.ShellIn) app.ShellIn {
	if rail, ok := app.RailFrom(ctx); ok {
		in.Chat = rail.Chat
		in.ChatOpen = rail.Open
	}
	return in
}
