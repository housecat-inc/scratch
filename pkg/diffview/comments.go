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
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	_ "modernc.org/sqlite"
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
	for _, stmt := range commentMigrations {
		if _, err := db.Exec(stmt); err != nil && !isDuplicateColumn(err) {
			db.Close()
			return nil, errors.Wrapf(err, "migrate: %s", stmt)
		}
	}
	return &SQLiteCommentStore{db: db}, nil
}

func (s *SQLiteCommentStore) Add(slug, path, side string, line int, body string) (Comment, error) {
	now := time.Now().UTC()
	c := Comment{
		Body:    body,
		Created: now,
		ID:      newCommentID(),
		Line:    line,
		Path:    path,
		Side:    side,
		Slug:    slug,
		Updated: now,
	}
	_, err := s.db.Exec(
		`INSERT INTO comments (body, created, id, line, path, resolved, side, slug, updated) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		c.Body, c.Created.UnixNano(), c.ID, c.Line, c.Path, 0, c.Side, c.Slug, c.Updated.UnixNano(),
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

func (s *SQLiteCommentStore) Get(slug, id string) (Comment, error) {
	row := s.db.QueryRow(
		`SELECT body, created, id, line, path, resolved, side, slug, updated FROM comments WHERE id = ? AND slug = ?`,
		id, slug,
	)
	c, err := scanComment(row.Scan)
	if errors.Is(err, sql.ErrNoRows) {
		return Comment{}, errCommentNotFound
	}
	return c, err
}

func (s *SQLiteCommentStore) List(slug string) ([]Comment, error) {
	rows, err := s.db.Query(
		`SELECT body, created, id, line, path, resolved, side, slug, updated FROM comments WHERE slug = ? ORDER BY created ASC`,
		slug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "query comments")
	}
	defer rows.Close()
	var out []Comment
	for rows.Next() {
		c, err := scanComment(rows.Scan)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "iterate comments")
	}
	return out, nil
}

func (s *SQLiteCommentStore) SetResolved(slug, id string, resolved bool) (Comment, error) {
	flag := 0
	if resolved {
		flag = 1
	}
	now := time.Now().UTC().UnixNano()
	res, err := s.db.Exec(
		`UPDATE comments SET resolved = ?, updated = ? WHERE id = ? AND slug = ?`,
		flag, now, id, slug,
	)
	if err != nil {
		return Comment{}, errors.Wrap(err, "update resolved")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Comment{}, errors.Wrap(err, "rows affected")
	}
	if n == 0 {
		return Comment{}, errCommentNotFound
	}
	return s.Get(slug, id)
}

func (s *SQLiteCommentStore) Update(slug, id, body string) (Comment, error) {
	now := time.Now().UTC().UnixNano()
	res, err := s.db.Exec(
		`UPDATE comments SET body = ?, updated = ? WHERE id = ? AND slug = ?`,
		body, now, id, slug,
	)
	if err != nil {
		return Comment{}, errors.Wrap(err, "update body")
	}
	n, err := res.RowsAffected()
	if err != nil {
		return Comment{}, errors.Wrap(err, "rows affected")
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
func (nopCommentStore) Delete(string, string) error             { return errCommentNotFound }
func (nopCommentStore) Get(string, string) (Comment, error)     { return Comment{}, errCommentNotFound }
func (nopCommentStore) List(string) ([]Comment, error)          { return nil, nil }
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

func isDuplicateColumn(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate column")
}

func newCommentID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}

func scanComment(scan func(dest ...any) error) (Comment, error) {
	var c Comment
	var createdNanos, updatedNanos int64
	var resolved int
	if err := scan(&c.Body, &createdNanos, &c.ID, &c.Line, &c.Path, &resolved, &c.Side, &c.Slug, &updatedNanos); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Comment{}, err
		}
		return Comment{}, errors.Wrap(err, "scan comment")
	}
	c.Created = time.Unix(0, createdNanos).UTC()
	c.Resolved = resolved != 0
	c.Updated = time.Unix(0, updatedNanos).UTC()
	return c, nil
}

const commentSchema = `
CREATE TABLE IF NOT EXISTS comments (
  body     TEXT    NOT NULL,
  created  INTEGER NOT NULL,
  id       TEXT    PRIMARY KEY,
  line     INTEGER NOT NULL,
  path     TEXT    NOT NULL,
  resolved INTEGER NOT NULL DEFAULT 0,
  side     TEXT    NOT NULL,
  slug     TEXT    NOT NULL,
  updated  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_comments_slug ON comments(slug);
`

var commentMigrations = []string{
	`ALTER TABLE comments ADD COLUMN resolved INTEGER NOT NULL DEFAULT 0`,
	`ALTER TABLE comments ADD COLUMN updated INTEGER NOT NULL DEFAULT 0`,
}
