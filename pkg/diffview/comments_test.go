package diffview

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteCommentStore {
	t.Helper()
	store, err := OpenSQLiteCommentStore(filepath.Join(t.TempDir(), "comments.db"))
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteCommentStoreCRUD(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store := newTestStore(t)

	c1, err := store.Add("acme/alpha", "foo.txt", "new", 12, "looks off")
	r.NoError(err)
	a.NotEmpty(c1.ID)
	a.Equal("foo.txt", c1.Path)
	a.Equal("new", c1.Side)
	a.Equal(12, c1.Line)
	a.Equal("looks off", c1.Body)

	c2, err := store.Add("acme/alpha", "foo.txt", "old", 5, "remove this?")
	r.NoError(err)
	_, err = store.Add("acme/beta", "bar.txt", "new", 1, "different repo")
	r.NoError(err)

	list, err := store.List("acme/alpha")
	r.NoError(err)
	r.Len(list, 2)
	a.Equal(c1.ID, list[0].ID)
	a.Equal(c2.ID, list[1].ID)

	r.NoError(store.Delete("acme/alpha", c1.ID))
	list, err = store.List("acme/alpha")
	r.NoError(err)
	r.Len(list, 1)
	a.Equal(c2.ID, list[0].ID)
}

func TestSQLiteCommentStoreDeleteMissing(t *testing.T) {
	tests := []struct {
		name string
		slug string
		id   string
	}{
		{name: "unknown id", slug: "acme/alpha", id: "deadbeef"},
		{name: "wrong slug", slug: "wrong/slug", id: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			store := newTestStore(t)
			c, err := store.Add("acme/alpha", "foo.txt", "new", 1, "x")
			r.NoError(err)

			id := tc.id
			if id == "" {
				id = c.ID
			}
			err = store.Delete(tc.slug, id)
			a.True(IsCommentNotFound(err), "expected not-found, got %v", err)
		})
	}
}

func TestCommentAnchorStable(t *testing.T) {
	a := assert.New(t)
	a.Equal(commentAnchor("foo.txt", "new", 12), commentAnchor("foo.txt", "new", 12))
	a.NotEqual(commentAnchor("foo.txt", "new", 12), commentAnchor("foo.txt", "old", 12))
	a.NotEqual(commentAnchor("foo.txt", "new", 12), commentAnchor("foo.txt", "new", 13))
}
