package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPagesPersist(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "scratch.db")
	store, err := New(path)
	r.NoError(err)
	pages, err := store.ListPages()
	r.NoError(err)
	r.Len(pages, 13)
	home, err := store.GetPage("getting-started")
	r.NoError(err)
	a.True(home.Home)
	r.NoError(store.SetPageHome("example"))
	r.Error(store.SetPageHome("missing"))
	thread, err := store.AddThread(ThreadKindChat, "Edit page", "")
	r.NoError(err)
	r.NoError(store.SetPagePinned("example", false))
	r.NoError(store.SetPageThread("example", thread.ID))
	r.NoError(store.Close())
	store, err = New(path)
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	page, err := store.GetPage("example")
	r.NoError(err)
	a.False(page.Pinned)
	a.True(page.Home)
	home, err = store.GetPage("getting-started")
	r.NoError(err)
	a.False(home.Home)
	a.Equal(thread.ID, page.ThreadID)
	r.NoError(store.DeleteThread(thread.ID))
	page, err = store.GetPage("example")
	r.NoError(err)
	a.Zero(page.ThreadID)
	pages, err = store.ListPages()
	r.NoError(err)
	a.Len(pages, 13)
}
