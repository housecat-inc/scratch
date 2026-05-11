package db

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
)

type Comment struct {
	Body         string
	Branch       string
	CreatedAt    time.Time
	ID           string
	Line         int
	Path         string
	Resolved     bool
	ResolvedBody string
	Side         string
	Slug         string
	UpdatedAt    time.Time
}

func (c Comment) Anchor() string { return CommentAnchor(c.Path, c.Side, c.Line) }

func CommentAnchor(path, side string, line int) string {
	sum := sha1.Sum([]byte(path + "|" + side + "|" + strconv.Itoa(line)))
	return hex.EncodeToString(sum[:8])
}

type CommentStore interface {
	AddComment(slug, branch, path, side string, line int, body string) (Comment, error)
	DeleteComment(slug, id string) error
	GetComment(slug, id string) (Comment, error)
	ListComments(slug, branch string) ([]Comment, error)
	SetCommentResolved(slug, id string, resolved bool, body string) (Comment, error)
	UpdateCommentBody(slug, id, body string) (Comment, error)
}

var ErrCommentNotFound = errors.New("comment not found")

func IsCommentNotFound(err error) bool { return errors.Is(err, ErrCommentNotFound) }

type NopCommentStore struct{}

func (NopCommentStore) AddComment(string, string, string, string, int, string) (Comment, error) {
	return Comment{}, errors.New("comments disabled")
}
func (NopCommentStore) DeleteComment(string, string) error { return ErrCommentNotFound }
func (NopCommentStore) GetComment(string, string) (Comment, error) {
	return Comment{}, ErrCommentNotFound
}
func (NopCommentStore) ListComments(string, string) ([]Comment, error) { return nil, nil }
func (NopCommentStore) SetCommentResolved(string, string, bool, string) (Comment, error) {
	return Comment{}, ErrCommentNotFound
}
func (NopCommentStore) UpdateCommentBody(string, string, string) (Comment, error) {
	return Comment{}, ErrCommentNotFound
}

func (d *DB) AddComment(slug, branch, path, side string, line int, body string) (Comment, error) {
	row, err := d.queries.AddComment(context.Background(), sqlite.AddCommentParams{
		Body:   body,
		Branch: branch,
		ID:     newCommentID(),
		Line:   int64(line),
		Path:   path,
		Side:   side,
		Slug:   slug,
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "insert comment")
	}
	return fromSqliteComment(row), nil
}

func (d *DB) DeleteComment(slug, id string) error {
	n, err := d.queries.DeleteComment(context.Background(), sqlite.DeleteCommentParams{ID: id, Slug: slug})
	if err != nil {
		return errors.Wrap(err, "delete comment")
	}
	if n == 0 {
		return ErrCommentNotFound
	}
	return nil
}

func (d *DB) GetComment(slug, id string) (Comment, error) {
	row, err := d.queries.GetComment(context.Background(), sqlite.GetCommentParams{ID: id, Slug: slug})
	if errors.Is(err, sql.ErrNoRows) {
		return Comment{}, ErrCommentNotFound
	}
	if err != nil {
		return Comment{}, errors.Wrap(err, "get comment")
	}
	return fromSqliteComment(row), nil
}

func (d *DB) ListComments(slug, branch string) ([]Comment, error) {
	rows, err := d.queries.ListComments(context.Background(), sqlite.ListCommentsParams{Slug: slug, Branch: branch})
	if err != nil {
		return nil, errors.Wrap(err, "list comments")
	}
	out := make([]Comment, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteComment(r))
	}
	return out, nil
}

func (d *DB) SetCommentResolved(slug, id string, resolved bool, body string) (Comment, error) {
	flag := int64(0)
	if resolved {
		flag = 1
	}
	n, err := d.queries.SetCommentResolved(context.Background(), sqlite.SetCommentResolvedParams{
		ID:           id,
		Resolved:     flag,
		ResolvedBody: body,
		Slug:         slug,
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "set resolved")
	}
	if n == 0 {
		return Comment{}, ErrCommentNotFound
	}
	return d.GetComment(slug, id)
}

func (d *DB) UpdateCommentBody(slug, id, body string) (Comment, error) {
	n, err := d.queries.UpdateCommentBody(context.Background(), sqlite.UpdateCommentBodyParams{
		Body: body,
		ID:   id,
		Slug: slug,
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "update body")
	}
	if n == 0 {
		return Comment{}, ErrCommentNotFound
	}
	return d.GetComment(slug, id)
}

func fromSqliteComment(c sqlite.Comment) Comment {
	return Comment{
		Body:         c.Body,
		Branch:       c.Branch,
		CreatedAt:    c.CreatedAt,
		ID:           c.ID,
		Line:         int(c.Line),
		Path:         c.Path,
		Resolved:     c.Resolved != 0,
		ResolvedBody: c.ResolvedBody,
		Side:         c.Side,
		Slug:         c.Slug,
		UpdatedAt:    c.UpdatedAt,
	}
}

func newCommentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
