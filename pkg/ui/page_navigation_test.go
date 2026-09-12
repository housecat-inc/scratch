package ui

import (
	"bytes"
	"context"
	"regexp"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/uikit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSharedPageNavigation(t *testing.T) {
	store, err := db.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	pages, err := store.ListPages()
	require.NoError(t, err)
	for _, page := range pages {
		t.Run(page.ID, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			ctx := WithCurrentPage(WithPageNavigation(context.Background(), pages), pages, page.Href())
			r.NotNil(uikit.ShellHeader(ctx))
			var html bytes.Buffer
			r.NoError(uikit.Shell(uikit.ShellProps{}).Render(ctx, &html))
			a.Contains(html.String(), `aria-label="Page navigation"`)
			a.Contains(html.String(), `/pages/`+page.ID+`/home`)
			a.Contains(html.String(), `/pages/`+page.ID+`/pin`)
			a.NotContains(html.String(), `href="/"`)
			a.NotContains(html.String(), `href="/pages"`)
		})
	}
	for _, path := range []string{"/inbox/chats/1", "/inbox/tasks/1", "/inbox/workflows/new", "/files/read", "/code/org/repo/commits"} {
		t.Run(path, func(t *testing.T) {
			ctx := WithCurrentPage(context.Background(), pages, path)
			header := uikit.ShellHeader(ctx)
			require.NotNil(t, header)
			var html bytes.Buffer
			require.NoError(t, header.Render(ctx, &html))
			assert.NotContains(t, html.String(), `/pages/inbox/home`)
		})
	}
}

func TestSidebarKeepsSectionNavigation(t *testing.T) {
	a := assert.New(t)
	pages := []db.Page{{ID: "chats", Pinned: false, Title: "Chats"}, {ID: "news", Pinned: true, Title: "News"}, {ID: "workflows", Pinned: true, Title: "Workflows"}}
	items := pageNavItems(pages, "workflows")
	var labels []string
	for _, item := range items {
		labels = append(labels, item.Label)
	}
	a.Equal([]string{"Home", "Pages", "News", "Chats", "Workflows", "Tasks", "Getting Started", "Setup", "AGENTS.md"}, labels)
	a.True(items[2].Child)
	a.True(items[3].Section)
	a.True(items[3].Sidebar)
	a.True(items[4].Active)
	a.True(items[4].Section)
	a.Equal("play", items[4].Icon)
	a.Equal("/inbox/workflows/running-count", items[4].CountURL)
}

func TestCreationNavigationInitialSelection(t *testing.T) {
	for _, tc := range []struct {
		item string
		path string
		view string
	}{
		{item: "chat", path: "/inbox/chats/new", view: "chats"},
		{item: "page", path: "/inbox/chats/new?intent=page", view: "chats"},
		{item: "page", path: "/pages/new", view: "pages"},
		{item: "workflow", path: "/inbox/chats/new?intent=workflow", view: "chats"},
		{item: "workflow", path: "/inbox/workflows/new", view: "workflows"},
	} {
		t.Run(tc.path, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			ctx := WithCurrentPage(context.Background(), nil, tc.path)
			var html bytes.Buffer
			r.NoError(uikit.Shell(uikit.ShellProps{Nav: pageNavItems(nil, tc.view)}).Render(ctx, &html))
			selected := regexp.MustCompile(`<[^>]*class="[^"]*\bactive\b[^"]*"[^>]*>`).FindAllString(html.String(), -1)
			r.Len(selected, 1)
			a.Contains(selected[0], `data-new-item="`+tc.item+`"`)
		})
	}
}
