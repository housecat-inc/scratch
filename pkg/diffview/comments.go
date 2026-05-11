package diffview

import (
	"context"
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
)

type Comment struct {
	Body     string
	Created  time.Time
	ID       string
	Line     int
	Path     string
	Resolved bool
	Side     string
	Slug     string
	Updated  time.Time
}

func (c Comment) Anchor() string { return commentAnchor(c.Path, c.Side, c.Line) }

type CommentStore interface {
	Add(slug, path, side string, line int, body string) (Comment, error)
	Delete(slug, id string) error
	Get(slug, id string) (Comment, error)
	List(slug string) ([]Comment, error)
	SetResolved(slug, id string, resolved bool) (Comment, error)
	Update(slug, id, body string) (Comment, error)
}

type SQLiteCommentStore struct {
	conn    *sql.DB
	owned   bool
	queries *db.Queries
}

func NewSQLiteCommentStore(conn *sql.DB) *SQLiteCommentStore {
	return &SQLiteCommentStore{conn: conn, queries: db.New(conn)}
}

func OpenSQLiteCommentStore(path string) (*SQLiteCommentStore, error) {
	conn, err := db.Open(path)
	if err != nil {
		return nil, err
	}
	s := NewSQLiteCommentStore(conn)
	s.owned = true
	return s, nil
}

func (s *SQLiteCommentStore) Close() error {
	if s.owned {
		return s.conn.Close()
	}
	return nil
}

func (s *SQLiteCommentStore) Add(slug, path, side string, line int, body string) (Comment, error) {
	now := time.Now().UTC()
	row, err := s.queries.AddComment(context.Background(), db.AddCommentParams{
		Body:    body,
		Created: now.UnixNano(),
		ID:      newCommentID(),
		Line:    int64(line),
		Path:    path,
		Side:    side,
		Slug:    slug,
		Updated: now.UnixNano(),
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "insert comment")
	}
	return fromDBComment(row), nil
}

func (s *SQLiteCommentStore) Delete(slug, id string) error {
	n, err := s.queries.DeleteComment(context.Background(), db.DeleteCommentParams{ID: id, Slug: slug})
	if err != nil {
		return errors.Wrap(err, "delete comment")
	}
	if n == 0 {
		return errCommentNotFound
	}
	return nil
}

func (s *SQLiteCommentStore) Get(slug, id string) (Comment, error) {
	row, err := s.queries.GetComment(context.Background(), db.GetCommentParams{ID: id, Slug: slug})
	if errors.Is(err, sql.ErrNoRows) {
		return Comment{}, errCommentNotFound
	}
	if err != nil {
		return Comment{}, errors.Wrap(err, "get comment")
	}
	return fromDBComment(row), nil
}

func (s *SQLiteCommentStore) List(slug string) ([]Comment, error) {
	rows, err := s.queries.ListComments(context.Background(), slug)
	if err != nil {
		return nil, errors.Wrap(err, "list comments")
	}
	out := make([]Comment, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromDBComment(r))
	}
	return out, nil
}

func (s *SQLiteCommentStore) SetResolved(slug, id string, resolved bool) (Comment, error) {
	flag := int64(0)
	if resolved {
		flag = 1
	}
	n, err := s.queries.SetCommentResolved(context.Background(), db.SetCommentResolvedParams{
		ID:       id,
		Resolved: flag,
		Slug:     slug,
		Updated:  time.Now().UTC().UnixNano(),
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "set resolved")
	}
	if n == 0 {
		return Comment{}, errCommentNotFound
	}
	return s.Get(slug, id)
}

func (s *SQLiteCommentStore) Update(slug, id, body string) (Comment, error) {
	n, err := s.queries.UpdateCommentBody(context.Background(), db.UpdateCommentBodyParams{
		Body:    body,
		ID:      id,
		Slug:    slug,
		Updated: time.Now().UTC().UnixNano(),
	})
	if err != nil {
		return Comment{}, errors.Wrap(err, "update body")
	}
	if n == 0 {
		return Comment{}, errCommentNotFound
	}
	return s.Get(slug, id)
}

var errCommentNotFound = errors.New("comment not found")

func IsCommentNotFound(err error) bool { return errors.Is(err, errCommentNotFound) }

type nopCommentStore struct{}

func (nopCommentStore) Add(string, string, string, int, string) (Comment, error) {
	return Comment{}, errors.New("comments disabled")
}
func (nopCommentStore) Delete(string, string) error         { return errCommentNotFound }
func (nopCommentStore) Get(string, string) (Comment, error) { return Comment{}, errCommentNotFound }
func (nopCommentStore) List(string) ([]Comment, error)      { return nil, nil }
func (nopCommentStore) SetResolved(string, string, bool) (Comment, error) {
	return Comment{}, errCommentNotFound
}
func (nopCommentStore) Update(string, string, string) (Comment, error) {
	return Comment{}, errCommentNotFound
}

func commentAnchor(path, side string, line int) string {
	sum := sha1.Sum([]byte(path + "|" + side + "|" + strconv.Itoa(line)))
	return hex.EncodeToString(sum[:8])
}

func fromDBComment(c db.Comment) Comment {
	return Comment{
		Body:     c.Body,
		Created:  time.Unix(0, c.Created).UTC(),
		ID:       c.ID,
		Line:     int(c.Line),
		Path:     c.Path,
		Resolved: c.Resolved != 0,
		Side:     c.Side,
		Slug:     c.Slug,
		Updated:  time.Unix(0, c.Updated).UTC(),
	}
}

func newCommentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
