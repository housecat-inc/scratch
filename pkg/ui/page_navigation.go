package ui

import (
	"context"
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/uikit"
)

type pageNavigationKey struct{}

func WithPageNavigation(ctx context.Context, pages []db.Page) context.Context {
	return context.WithValue(ctx, pageNavigationKey{}, pages)
}

func pageNavItems(pages []db.Page, active string) []uikit.NavItem {
	for _, page := range pages {
		if page.Home && page.ID == active {
			active = "home"
			break
		}
	}
	items := []uikit.NavItem{{Active: active == "home", Href: "/", Icon: "home", Label: "Home", Section: true}, {Active: active == "pages", Href: "/pages", Icon: "file", Label: "Pages", Section: true}}
	for _, page := range pages {
		switch page.ID {
		case "agents", "chats", "getting-started", "pages", "setup", "tasks", "workflows":
			continue
		}
		if page.Pinned {
			items = append(items, uikit.NavItem{Active: page.ID == active, Child: true, Href: page.Href(), Label: page.Title})
		}
	}
	return append(items, []uikit.NavItem{
		{Active: active == "chats", Href: "/inbox/chats", Icon: "chat", Label: "Chats", Section: true, Sidebar: true},
		{Active: active == "workflows", CountURL: "/inbox/workflows/running-count", Href: "/inbox/workflows", Icon: "play", Label: "Workflows", Section: true},
		{Active: active == "tasks", Href: "/inbox/tasks", Icon: "task", Label: "Tasks", Section: true},
		{Active: active == "getting-started", Href: "/getting-started", Icon: "file", Label: "Getting Started", Section: true},
		{Active: active == "setup", Child: true, Href: "/setup", Label: "Setup"},
		{Active: active == "agents", Child: true, Href: "/files/view?path=AGENTS.md", Label: "AGENTS.md"},
	}...)
}

func homeNavigationActive(ctx context.Context, active string) bool {
	pages, _ := ctx.Value(pageNavigationKey{}).([]db.Page)
	for _, page := range pages {
		if page.Home && page.ID == active {
			return true
		}
	}
	return false
}

func navigationPages(ctx context.Context) []db.Page {
	pages, _ := ctx.Value(pageNavigationKey{}).([]db.Page)
	return pages
}

func WithCurrentPage(ctx context.Context, pages []db.Page, path string) context.Context {
	ctx = uikit.WithCreationNavigation(ctx, path)
	for _, page := range pages {
		if path == page.Href() {
			return uikit.WithShellHeader(ctx, PageHeader(page))
		}
	}
	path = strings.SplitN(path, "?", 2)[0]
	var current *db.Page
	for _, page := range pages {
		href := strings.TrimSuffix(page.Href(), "/")
		if path == href || strings.HasPrefix(path, href+"/") && page.ID != "pages" {
			if current == nil || len(href) > len(strings.TrimSuffix(current.Href(), "/")) {
				current = &page
			}
		}
	}
	if current != nil {
		return uikit.WithShellHeader(ctx, PageHeader(*current))
	}
	return ctx
}
