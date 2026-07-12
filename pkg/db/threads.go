package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

const (
	ThreadKindChat   = "chat"
	ThreadKindEmail  = "email"
	ThreadKindReview = "review"

	ThreadStateArchived = "archived"
	ThreadStateOpen     = "open"
	ThreadStateResolved = "resolved"
)

type Thread struct {
	Anchor    string
	CreatedAt ts.Timestamp
	ID        int64
	Kind      string
	State     string
	Title     string
	UpdatedAt ts.Timestamp
}

type ThreadStore interface {
	AddMessage(m NewMessage) (Message, error)
	AddMessageEvent(messageID int64, eventType, data string) (MessageEvent, error)
	AddThread(kind, title, anchor string) (Thread, error)
	AppendMessageBody(id int64, delta string) error
	FinishMessage(id int64, status string) (Message, error)
	GetMessage(id int64) (Message, error)
	GetThread(id int64) (Thread, error)
	ListMessageEvents(messageID, afterSeq int64) ([]MessageEvent, error)
	ListThreadMessages(threadID int64) ([]Message, error)
	ListThreads(kind string) ([]Thread, error)
	SetThreadAnchor(id int64, anchor string) error
	SetThreadState(id int64, state string) error
	SetThreadTitle(id int64, title string) error
	TouchThread(id int64) error
}

var ErrThreadNotFound = errors.New("thread not found")

func IsThreadNotFound(err error) bool { return errors.Is(err, ErrThreadNotFound) }

func (d *DB) AddThread(kind, title, anchor string) (Thread, error) {
	if anchor == "" {
		anchor = "{}"
	}
	now := ts.Now()
	row, err := d.queries.AddThread(context.Background(), sqlite.AddThreadParams{
		AnchorJson: anchor,
		CreatedAt:  now,
		Kind:       kind,
		State:      ThreadStateOpen,
		Title:      title,
		UpdatedAt:  now,
	})
	if err != nil {
		return Thread{}, errors.Wrap(err, "insert thread")
	}
	return fromSqliteThread(row), nil
}

func (d *DB) GetThread(id int64) (Thread, error) {
	row, err := d.queries.GetThread(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Thread{}, ErrThreadNotFound
	}
	if err != nil {
		return Thread{}, errors.Wrap(err, "get thread")
	}
	return fromSqliteThread(row), nil
}

func (d *DB) ListThreads(kind string) ([]Thread, error) {
	rows, err := d.queries.ListThreads(context.Background(), kind)
	if err != nil {
		return nil, errors.Wrap(err, "list threads")
	}
	out := make([]Thread, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteThread(r))
	}
	return out, nil
}

func (d *DB) SetThreadAnchor(id int64, anchor string) error {
	n, err := d.queries.SetThreadAnchor(context.Background(), sqlite.SetThreadAnchorParams{
		AnchorJson: anchor,
		ID:         id,
		UpdatedAt:  ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "set thread anchor")
	}
	if n == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func (d *DB) SetThreadState(id int64, state string) error {
	n, err := d.queries.SetThreadState(context.Background(), sqlite.SetThreadStateParams{
		ID:        id,
		State:     state,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "set thread state")
	}
	if n == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func (d *DB) SetThreadTitle(id int64, title string) error {
	n, err := d.queries.SetThreadTitle(context.Background(), sqlite.SetThreadTitleParams{
		ID:        id,
		Title:     title,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "set thread title")
	}
	if n == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func (d *DB) TouchThread(id int64) error {
	n, err := d.queries.TouchThread(context.Background(), sqlite.TouchThreadParams{
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "touch thread")
	}
	if n == 0 {
		return ErrThreadNotFound
	}
	return nil
}

func fromSqliteThread(t sqlite.Thread) Thread {
	return Thread{
		Anchor:    t.AnchorJson,
		CreatedAt: t.CreatedAt,
		ID:        t.ID,
		Kind:      t.Kind,
		State:     t.State,
		Title:     t.Title,
		UpdatedAt: t.UpdatedAt,
	}
}
