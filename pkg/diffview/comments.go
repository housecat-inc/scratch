package diffview

import (
	"crypto/rand"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	_ "modernc.org/sqlite"
)

type Comment struct {
	Body    string
	Created time.Time
	ID      string
	Line    int
	Path    string
	Side    string
	Slug    string
}

func (c Comment) Anchor() string { return commentAnchor(c.Path, c.Side, c.Line) }

type CommentStore interface {
	Add(slug, path, side string, line int, body string) (Comment, error)
	Delete(slug, id string) error
	List(slug string) ([]Comment, error)
}

type SQLiteCommentStore struct {
	db *sql.DB
}

func OpenSQLiteCommentStore(path string) (*SQLiteCommentStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, errors.Wrapf(err, "mkdir %s", dir)
		}
	}
	dsn := commentDSN(path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, errors.Wrap(err, "open sqlite")
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(commentSchema); err != nil {
		db.Close()
		return nil, errors.Wrap(err, "migrate")
	}
	return &SQLiteCommentStore{db: db}, nil
}

func (s *SQLiteCommentStore) Add(slug, path, side string, line int, body string) (Comment, error) {
	c := Comment{
		Body:    body,
		Created: time.Now().UTC(),
		ID:      newCommentID(),
		Line:    line,
		Path:    path,
		Side:    side,
		Slug:    slug,
	}
	_, err := s.db.Exec(
		`INSERT INTO comments (body, created, id, line, path, side, slug) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		c.Body, c.Created.UnixNano(), c.ID, c.Line, c.Path, c.Side, c.Slug,
	)
	if err != nil {
		return Comment{}, errors.Wrap(err, "insert comment")
	}
	return c, nil
}

func (s *SQLiteCommentStore) Close() error { return s.db.Close() }

func (s *SQLiteCommentStore) Delete(slug, id string) error {
	res, err := s.db.Exec(`DELETE FROM comments WHERE id = ? AND slug = ?`, id, slug)
	if err != nil {
		return errors.Wrap(err, "delete comment")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return errors.Wrap(err, "rows affected")
	}
	if n == 0 {
		return errCommentNotFound
	}
	return nil
}

func (s *SQLiteCommentStore) List(slug string) ([]Comment, error) {
	rows, err := s.db.Query(
		`SELECT body, created, id, line, path, side, slug FROM comments WHERE slug = ? ORDER BY created ASC`,
		slug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "query comments")
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		var c Comment
		var createdNanos int64
		if err := rows.Scan(&c.Body, &createdNanos, &c.ID, &c.Line, &c.Path, &c.Side, &c.Slug); err != nil {
			return nil, errors.Wrap(err, "scan comment")
		}
		c.Created = time.Unix(0, createdNanos).UTC()
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate comments")
	}
	return out, nil
}

var errCommentNotFound = errors.New("comment not found")

func IsCommentNotFound(err error) bool { return errors.Is(err, errCommentNotFound) }

type nopCommentStore struct{}

func (nopCommentStore) Add(string, string, string, int, string) (Comment, error) {
	return Comment{}, errors.New("comments disabled")
}
func (nopCommentStore) Delete(string, string) error  { return errCommentNotFound }
func (nopCommentStore) List(string) ([]Comment, error) { return nil, nil }

func commentAnchor(path, side string, line int) string {
	sum := sha1.Sum([]byte(path + "|" + side + "|" + strconv.Itoa(line)))
	return hex.EncodeToString(sum[:8])
}

func commentDSN(path string) string {
	if path == ":memory:" {
		return path
	}
	v := url.Values{}
	v.Set("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "foreign_keys(on)")
	return "file:" + path + "?" + v.Encode()
}

func newCommentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

const commentSchema = `
CREATE TABLE IF NOT EXISTS comments (
  body    TEXT    NOT NULL,
  created INTEGER NOT NULL,
  id      TEXT    PRIMARY KEY,
  line    INTEGER NOT NULL,
  path    TEXT    NOT NULL,
  side    TEXT    NOT NULL,
  slug    TEXT    NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_comments_slug ON comments(slug);
`
