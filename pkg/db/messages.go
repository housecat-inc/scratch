package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

const (
	MessageRoleAssistant = "assistant"
	MessageRoleSystem    = "system"
	MessageRoleUser      = "user"

	MessageStatusComplete  = "complete"
	MessageStatusDraft     = "draft"
	MessageStatusError     = "error"
	MessageStatusStreaming = "streaming"
)

type Message struct {
	Author      string
	Body        string
	CompletedAt *ts.Timestamp
	CreatedAt   ts.Timestamp
	ID          int64
	Meta        string
	ParentID    *int64
	ReadAt      *ts.Timestamp
	Role        string
	Seq         int64
	Status      string
	ThreadID    int64
	UpdatedAt   ts.Timestamp
}

type MessageEvent struct {
	CreatedAt ts.Timestamp
	Data      string
	ID        int64
	MessageID int64
	Seq       int64
	Type      string
}

type NewMessage struct {
	Author   string
	Body     string
	Meta     string
	ParentID *int64
	Role     string
	Status   string
	ThreadID int64
}

var ErrMessageNotFound = errors.New("message not found")

func IsMessageNotFound(err error) bool { return errors.Is(err, ErrMessageNotFound) }

func (d *DB) AddMessage(m NewMessage) (Message, error) {
	if m.Meta == "" {
		m.Meta = "{}"
	}
	if m.Status == "" {
		m.Status = MessageStatusComplete
	}
	parentID := sql.NullInt64{}
	if m.ParentID != nil {
		parentID = sql.NullInt64{Int64: *m.ParentID, Valid: true}
	}
	now := ts.Now()
	row, err := d.queries.AddMessage(context.Background(), sqlite.AddMessageParams{
		Author:    m.Author,
		Body:      m.Body,
		CreatedAt: now,
		MetaJson:  m.Meta,
		ParentID:  parentID,
		Role:      m.Role,
		Status:    m.Status,
		ThreadID:  m.ThreadID,
		UpdatedAt: now,
	})
	if err != nil {
		return Message{}, errors.Wrap(err, "insert message")
	}
	return fromSqliteMessage(row), nil
}

func (d *DB) AddMessageEvent(messageID int64, eventType, data string) (MessageEvent, error) {
	row, err := d.queries.AddMessageEvent(context.Background(), sqlite.AddMessageEventParams{
		CreatedAt: ts.Now(),
		DataJson:  data,
		MessageID: messageID,
		Type:      eventType,
	})
	if err != nil {
		return MessageEvent{}, errors.Wrap(err, "insert message event")
	}
	return fromSqliteMessageEvent(row), nil
}

func (d *DB) AppendMessageBody(id int64, delta string) error {
	n, err := d.queries.AppendMessageBody(context.Background(), sqlite.AppendMessageBodyParams{
		Body:      delta,
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "append message body")
	}
	if n == 0 {
		return ErrMessageNotFound
	}
	return nil
}

func (d *DB) FinishMessage(id int64, status string) (Message, error) {
	n, err := d.queries.FinishMessage(context.Background(), sqlite.FinishMessageParams{
		CompletedAt: ts.NowPtr(),
		ID:          id,
		Status:      status,
		UpdatedAt:   ts.Now(),
	})
	if err != nil {
		return Message{}, errors.Wrap(err, "finish message")
	}
	if n == 0 {
		return Message{}, ErrMessageNotFound
	}
	return d.GetMessage(id)
}

func (d *DB) ResetMessageEvents(id int64) error {
	return errors.Wrap(d.queries.ResetMessageEvents(context.Background(), id), "reset message events")
}

func (d *DB) SetMessageBody(id int64, body string) error {
	n, err := d.queries.SetMessageBody(context.Background(), sqlite.SetMessageBodyParams{
		Body:      body,
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "set message body")
	}
	if n == 0 {
		return ErrMessageNotFound
	}
	return nil
}

func (d *DB) SetMessageMeta(id int64, meta string) error {
	n, err := d.queries.SetMessageMeta(context.Background(), sqlite.SetMessageMetaParams{
		ID:        id,
		MetaJson:  meta,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return errors.Wrap(err, "set message meta")
	}
	if n == 0 {
		return ErrMessageNotFound
	}
	return nil
}

func (d *DB) GetMessage(id int64) (Message, error) {
	row, err := d.queries.GetMessage(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Message{}, ErrMessageNotFound
	}
	if err != nil {
		return Message{}, errors.Wrap(err, "get message")
	}
	return fromSqliteMessage(row), nil
}

func (d *DB) ListMessageEvents(messageID, afterSeq int64) ([]MessageEvent, error) {
	rows, err := d.queries.ListMessageEvents(context.Background(), sqlite.ListMessageEventsParams{
		MessageID: messageID,
		Seq:       afterSeq,
	})
	if err != nil {
		return nil, errors.Wrap(err, "list message events")
	}
	out := make([]MessageEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteMessageEvent(r))
	}
	return out, nil
}

func (d *DB) ListMessagesByStatus(status string) ([]Message, error) {
	rows, err := d.queries.ListMessagesByStatus(context.Background(), status)
	if err != nil {
		return nil, errors.Wrap(err, "list messages by status")
	}
	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteMessage(r))
	}
	return out, nil
}

func (d *DB) ListThreadMessageEvents(threadID int64) ([]MessageEvent, error) {
	rows, err := d.queries.ListThreadMessageEvents(context.Background(), threadID)
	if err != nil {
		return nil, errors.Wrap(err, "list thread message events")
	}
	out := make([]MessageEvent, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteMessageEvent(r))
	}
	return out, nil
}

func (d *DB) ListThreadMessages(threadID int64) ([]Message, error) {
	rows, err := d.queries.ListThreadMessages(context.Background(), threadID)
	if err != nil {
		return nil, errors.Wrap(err, "list thread messages")
	}
	out := make([]Message, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteMessage(r))
	}
	return out, nil
}

func fromSqliteMessage(m sqlite.Message) Message {
	var parentID *int64
	if m.ParentID.Valid {
		parentID = &m.ParentID.Int64
	}
	return Message{
		Author:      m.Author,
		Body:        m.Body,
		CompletedAt: m.CompletedAt,
		CreatedAt:   m.CreatedAt,
		ID:          m.ID,
		Meta:        m.MetaJson,
		ParentID:    parentID,
		ReadAt:      m.ReadAt,
		Role:        m.Role,
		Seq:         m.Seq,
		Status:      m.Status,
		ThreadID:    m.ThreadID,
		UpdatedAt:   m.UpdatedAt,
	}
}

func fromSqliteMessageEvent(e sqlite.MessageEvent) MessageEvent {
	return MessageEvent{
		CreatedAt: e.CreatedAt,
		Data:      e.DataJson,
		ID:        e.ID,
		MessageID: e.MessageID,
		Seq:       e.Seq,
		Type:      e.Type,
	}
}
