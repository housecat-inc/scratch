package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
)

type Page struct {
	Description string
	Home        bool
	ID          string
	Pinned      bool
	ThreadID    int64
	Title       string
}

type PageStore interface {
	GetPage(string) (Page, error)
	GetPageByThread(int64) (Page, error)
	ListPages() ([]Page, error)
	SetPageHome(string) error
	SetPagePinned(string, bool) error
	SetPageThread(string, int64) error
}

func (d *DB) GetPage(id string) (Page, error) {
	row, err := d.queries.GetPage(context.Background(), id)
	return fromSqlitePage(row), errors.Wrap(err, "get page")
}

func (d *DB) GetPageByThread(id int64) (Page, error) {
	row, err := d.queries.GetPageByThread(context.Background(), sql.NullInt64{Int64: id, Valid: true})
	return fromSqlitePage(row), errors.Wrap(err, "get page by thread")
}

func (d *DB) ListPages() ([]Page, error) {
	rows, err := d.queries.ListPages(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "list pages")
	}
	pages := make([]Page, 0, len(rows))
	for _, row := range rows {
		pages = append(pages, fromSqlitePage(row))
	}
	return pages, nil
}

func (d *DB) SetPagePinned(id string, pinned bool) error {
	n, err := d.queries.SetPagePinned(context.Background(), sqlite.SetPagePinnedParams{ID: id, Pinned: pinned})
	if err != nil {
		return errors.Wrap(err, "pin page")
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (d *DB) SetPageThread(id string, threadID int64) error {
	n, err := d.queries.SetPageThread(context.Background(), sqlite.SetPageThreadParams{ID: id, ThreadID: sql.NullInt64{Int64: threadID, Valid: true}})
	if err != nil {
		return errors.Wrap(err, "link page chat")
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func fromSqlitePage(row sqlite.Page) Page {
	return Page{Description: row.Description, Home: row.Home, ID: row.ID, Pinned: row.Pinned, ThreadID: row.ThreadID.Int64, Title: row.Title}
}

func (d *DB) SetPageHome(id string) error {
	n, err := d.queries.SetPageHome(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "pin home page")
	}
	if n == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (p Page) Href() string {
	switch p.ID {
	case "agents":
		return "/files/view?path=AGENTS.md"
	case "chats", "tasks", "workflows":
		return "/inbox/" + p.ID
	case "code", "getting-started", "inbox", "pages", "sessions", "setup", "starred":
		return "/" + p.ID
	case "files":
		return "/files/"
	default:
		return "/pages/" + p.ID
	}
}

func (p Page) BuiltIn() bool {
	switch p.ID {
	case "agents", "chats", "code", "files", "getting-started", "inbox", "pages", "sessions", "setup", "starred", "tasks", "workflows":
		return true
	default:
		return false
	}
}
