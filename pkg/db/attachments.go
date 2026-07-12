package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

type Attachment struct {
	CreatedAt ts.Timestamp
	FilePath  string
	ID        int64
	MessageID *int64
	MimeType  string
	Name      string
	Size      int64
	ThreadID  int64
}

type NewAttachment struct {
	FilePath string
	MimeType string
	Name     string
	Size     int64
	ThreadID int64
}

var ErrAttachmentNotFound = errors.New("attachment not found")

func IsAttachmentNotFound(err error) bool { return errors.Is(err, ErrAttachmentNotFound) }

func (d *DB) AddAttachment(a NewAttachment) (Attachment, error) {
	row, err := d.queries.AddAttachment(context.Background(), sqlite.AddAttachmentParams{
		CreatedAt: ts.Now(),
		FilePath:  a.FilePath,
		MimeType:  a.MimeType,
		Name:      a.Name,
		Size:      a.Size,
		ThreadID:  a.ThreadID,
	})
	if err != nil {
		return Attachment{}, errors.Wrap(err, "insert attachment")
	}
	return fromSqliteAttachment(row), nil
}

func (d *DB) DeleteDraftAttachment(id int64) (Attachment, error) {
	row, err := d.queries.DeleteDraftAttachment(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, errors.Wrap(err, "delete draft attachment")
	}
	return fromSqliteAttachment(row), nil
}

func (d *DB) GetAttachment(id int64) (Attachment, error) {
	row, err := d.queries.GetAttachment(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Attachment{}, ErrAttachmentNotFound
	}
	if err != nil {
		return Attachment{}, errors.Wrap(err, "get attachment")
	}
	return fromSqliteAttachment(row), nil
}

func (d *DB) LinkAttachments(threadID, messageID int64, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	err := d.queries.LinkAttachments(context.Background(), sqlite.LinkAttachmentsParams{
		Ids:       ids,
		MessageID: sql.NullInt64{Int64: messageID, Valid: true},
		ThreadID:  threadID,
	})
	return errors.Wrap(err, "link attachments")
}

func (d *DB) ListMessageAttachments(messageID int64) ([]Attachment, error) {
	rows, err := d.queries.ListMessageAttachments(context.Background(), sql.NullInt64{Int64: messageID, Valid: true})
	if err != nil {
		return nil, errors.Wrap(err, "list message attachments")
	}
	out := make([]Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteAttachment(r))
	}
	return out, nil
}

func (d *DB) ListThreadAttachments(threadID int64) ([]Attachment, error) {
	rows, err := d.queries.ListThreadAttachments(context.Background(), threadID)
	if err != nil {
		return nil, errors.Wrap(err, "list thread attachments")
	}
	out := make([]Attachment, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteAttachment(r))
	}
	return out, nil
}

func fromSqliteAttachment(a sqlite.Attachment) Attachment {
	att := Attachment{
		CreatedAt: a.CreatedAt,
		FilePath:  a.FilePath,
		ID:        a.ID,
		MimeType:  a.MimeType,
		Name:      a.Name,
		Size:      a.Size,
		ThreadID:  a.ThreadID,
	}
	if a.MessageID.Valid {
		att.MessageID = &a.MessageID.Int64
	}
	return att
}
