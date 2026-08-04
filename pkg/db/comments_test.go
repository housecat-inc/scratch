package db

import (
	"path/filepath"
	"testing"
	"time"

	tk "github.com/housecat-inc/scratch/testkit/v2"
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

func TestCommentCRUD(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	d := newTestDB(t)

	c1, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 12, "looks off")
	r.NoError(err)
	a.NotEmpty(c1.ID)
	a.Equal("feature", c1.Branch)
	a.Equal("foo.txt", c1.Path)
	a.Equal("new", c1.Side)
	a.Equal(12, c1.Line)
	a.Equal("looks off", c1.Body)
	a.False(c1.Resolved)
	a.False(c1.CreatedAt.IsZero())

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

	r.NoError(d.DeleteComment("acme/alpha", c1.ID))
	list, err = d.ListComments("acme/alpha", "feature")
	r.NoError(err)
	r.Len(list, 1)
	a.Equal(c2.ID, list[0].ID)
}

func TestCommentDeleteMissing(t *testing.T) {
	type fixture struct {
		db       *DB
		seededID string
	}
	type in struct{ slug, id string }

	notFound := func(t *tk.T, _ struct{}, err error) {
		t.A.True(IsCommentNotFound(err), "expected not-found, got %v", err)
	}

	tk.RunF(t, []tk.Test[in, struct{}]{
		{Name: "unknown id", In: in{slug: "acme/alpha", id: "deadbeef"}, Assert: notFound},
		{Name: "wrong slug", In: in{slug: "wrong/slug"}, Assert: notFound},
	}, func(t *tk.T) fixture {
		d := newTestDB(t.T)
		c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 1, "x")
		t.R.NoError(err)
		return fixture{db: d, seededID: c.ID}
	}, func(t *tk.T, f fixture, i in) (struct{}, error) {
		id := i.id
		if id == "" {
			id = f.seededID
		}
		return struct{}{}, f.db.DeleteComment(i.slug, id)
	})
}

func TestCommentUpdateAndResolve(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	d := newTestDB(t)
	c, err := d.AddComment("acme/alpha", "feature", "foo.txt", "new", 4, "first draft")
	r.NoError(err)
	a.False(c.Resolved)

	time.Sleep(time.Second) // ensure CURRENT_TIMESTAMP changes
	updated, err := d.UpdateCommentBody("acme/alpha", c.ID, "edited body")
	r.NoError(err)
	a.Equal("edited body", updated.Body)
	a.True(updated.UpdatedAt.After(c.CreatedAt.Add(-time.Second)))

	resolved, err := d.SetCommentResolved("acme/alpha", c.ID, true, "fixed in commit abc")
	r.NoError(err)
	a.True(resolved.Resolved)
	a.Equal("fixed in commit abc", resolved.ResolvedBody)
	a.Equal("edited body", resolved.Body)

	again, err := d.SetCommentResolved("acme/alpha", c.ID, false, "")
	r.NoError(err)
	a.False(again.Resolved)
	a.Empty(again.ResolvedBody)
}

func TestCommentGetMissing(t *testing.T) {
	a := assert.New(t)

	d := newTestDB(t)
	_, err := d.GetComment("acme/alpha", "nope")
	a.True(IsCommentNotFound(err))
}

func TestCommentAnchorStable(t *testing.T) {
	a := assert.New(t)
	a.Equal(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "new", 12))
	a.NotEqual(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "old", 12))
	a.NotEqual(CommentAnchor("foo.txt", "new", 12), CommentAnchor("foo.txt", "new", 13))
}
