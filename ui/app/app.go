package app

import (
	"net/url"

	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/icons"
)

const (
	AssetsPath       = "/assets/"
	ChatParam        = "chat"
	SidebarContentID = "mail-sidebar-content"
	SidebarDialogsID = "sidebar-dialogs"
	ThemeColor       = "#ffffff"
)

type NavItem struct {
	Count         string
	Current       bool
	Group         string
	Href          string
	Icon          icons.Name
	IconComponent templ.Component
	Label         string
}

type NavGroup struct {
	Items []NavItem
	Name  string
}

type ShellIn struct {
	BrandHref string
	BrandName string
	Chat      templ.Component
	ChatOpen  bool
	Head      templ.Component
	Nav       []NavItem
	NavHeader templ.Component
	Title     string
	TopBar    templ.Component
}

func groups(items []NavItem) []NavGroup {
	out := []NavGroup{}
	index := map[string]int{}
	for _, item := range items {
		i, ok := index[item.Group]
		if !ok {
			i = len(out)
			index[item.Group] = i
			out = append(out, NavGroup{Name: item.Group})
		}
		out[i].Items = append(out[i].Items, item)
	}
	return out
}

func chatHref(base string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	q := u.Query()
	q.Set(ChatParam, "new")
	u.RawQuery = q.Encode()
	return u.String()
}

func asset(name string) string {
	return AssetsPath + name + "?v=" + Version
}
