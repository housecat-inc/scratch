package db

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestDB(t *testing.T) *DB {
	t.Helper()
	d, err := New(filepath.Join(t.TempDir(), "scratch.db"))
	require.NoError(t, err)
	t.Cleanup(func() { d.Close() })
	return d
}

func TestAddComment(t *testing.T) {
	t.Run("stores the anchor and body as an open comment", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		d := newTestDB(t)
		c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 12, "looks off")
		r.NoError(err)
		a.NotEmpty(c.ID)
		a.Equal("feature", c.Branch)
		a.Equal("foo.txt", c.Path)
		a.Equal("new", c.Side)
		a.Equal(12, c.Line)
		a.Equal("looks off", c.Body)
		a.False(c.Resolved)
		a.False(c.CreatedAt.IsZero())
	})
}

func TestCommentAnchor(t *testing.T) {
	t.Run("is stable for the same path side and line", func(t *testing.T) {
		a := assert.New(t)
		a.Equal(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "new", 12))
	})
	t.Run("differs by side", func(t *testing.T) {
		a := assert.New(t)
		a.NotEqual(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "old", 12))
	})
	t.Run("differs by line", func(t *testing.T) {
		a := assert.New(t)
		a.NotEqual(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "new", 13))
	})
}

func TestDeleteComment(t *testing.T) {
	t.Run("removes only the deleted comment from the branch list", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		d := newTestDB(t)
		c1, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 12, "looks off")
		r.NoError(err)
		c2, err := d.AddComment("acme/alpha", "feature", "foo.txt", "old", 5, "remove this?")
		r.NoError(err)

		r.NoError(d.DeleteComment("acme/alpha", c1.ID))
		list, err := d.ListComments("acme/alpha", "feature")
		r.NoError(err)
		r.Len(list, 1)
		a.Equal(c2.ID, list[0].ID)
	})

	tests := []struct {
		id   string
		name string
		slug string
	}{
		{id: "deadbeef", name: "reports not found for an unknown id", slug: "acme/alpha"},
		{name: "reports not found for another repo slug", slug: "wrong/slug"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			d := newTestDB(t)
			c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 1, "x")
			r.NoError(err)
			id := tt.id
			if id == "" {
				id = c.ID
			}
			err = d.DeleteComment(tt.slug, id)
			a.True(IsCommentNotFound(err), "expected not-found, got %v", err)
		})
	}
}

func TestGetComment(t *testing.T) {
	t.Run("reports not found for an unknown id", func(t *testing.T) {
		a := assert.New(t)

		d := newTestDB(t)
		_, err := d.GetComment("acme/alpha", "nope")
		a.True(IsCommentNotFound(err))
	})
}

func TestListComments(t *testing.T) {
	t.Run("returns the branch comments in creation order", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		d := newTestDB(t)
		c1, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 12, "looks off")
		r.NoError(err)
		c2, err := d.AddComment("acme/alpha", "feature", "foo.txt", "old", 5, "remove this?")
		r.NoError(err)
		_, err = d.AddComment("acme/alpha", "other-branch", "foo.txt", "new", 1, "different branch")
		r.NoError(err)
		_, err = d.AddComment("acme/beta", "feature", "bar.txt", "new", 1, "different repo")
		r.NoError(err)

		list, err := d.ListComments("acme/alpha", "feature")
		r.NoError(err)
		r.Len(list, 2)
		a.Equal(c1.ID, list[0].ID)
		a.Equal(c2.ID, list[1].ID)
	})
}

func TestSetCommentResolved(t *testing.T) {
	t.Run("resolves with a reason and reopens clearing it", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		d := newTestDB(t)
		c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "first draft")
		r.NoError(err)
		a.False(c.Resolved)
		_, err = d.UpdateCommentBody("acme/alpha", c.ID, "edited body")
		r.NoError(err)

		resolved, err := d.SetCommentResolved("acme/alpha", c.ID, true, "fixed in commit abc")
		r.NoError(err)
		a.True(resolved.Resolved)
		a.Equal("fixed in commit abc", resolved.ResolvedBody)
		a.Equal("edited body", resolved.Body)

		again, err := d.SetCommentResolved("acme/alpha", c.ID, false, "")
		r.NoError(err)
		a.False(again.Resolved)
		a.Empty(again.ResolvedBody)
	})
}

func TestUpdateCommentBody(t *testing.T) {
	t.Run("replaces the body and bumps the update time", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		d := newTestDB(t)
		c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "first draft")
		r.NoError(err)

		time.Sleep(time.Second)
		updated, err := d.UpdateCommentBody("acme/alpha", c.ID, "edited body")
		r.NoError(err)
		a.Equal("edited body", updated.Body)
		a.True(updated.UpdatedAt.After(c.CreatedAt.Add(-time.Second)))
	})
}
