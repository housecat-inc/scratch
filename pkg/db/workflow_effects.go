package db

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

func (d *DB) AddWorkflowTask(key, title string) (Task, error) {
	ctx := context.Background()
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return Task{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO workflow_effects(effect_key) VALUES (?)", key)
	if err != nil {
		return Task{}, err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		var raw string
		if err = tx.QueryRowContext(ctx, "SELECT result_json FROM workflow_effects WHERE effect_key=?", key).Scan(&raw); err != nil {
			return Task{}, err
		}
		var task Task
		err = json.Unmarshal([]byte(raw), &task)
		return task, err
	}
	now := ts.Now()
	row, err := d.queries.WithTx(tx).AddTask(ctx, sqlite.AddTaskParams{CreatedAt: now, Title: title, UpdatedAt: now})
	if err != nil {
		return Task{}, err
	}
	task := fromSqliteTask(row)
	raw, err := json.Marshal(task)
	if err != nil {
		return Task{}, err
	}
	if _, err = tx.ExecContext(ctx, "UPDATE workflow_effects SET result_json=? WHERE effect_key=?", string(raw), key); err != nil {
		return Task{}, err
	}
	return task, tx.Commit()
}
func (d *DB) AddWorkflowMessageEvent(key string, messageID int64, eventType, data, delta string) error {
	ctx := context.Background()
	tx, err := d.conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT OR IGNORE INTO workflow_effects(effect_key) VALUES (?)", key)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return nil
	}
	q := d.queries.WithTx(tx)
	if _, err = q.AddMessageEvent(ctx, sqlite.AddMessageEventParams{CreatedAt: ts.Now(), DataJson: data, MessageID: messageID, Type: eventType}); err != nil {
		return err
	}
	if delta != "" {
		n, err := q.AppendMessageBody(ctx, sqlite.AppendMessageBodyParams{Body: delta, ID: messageID, UpdatedAt: ts.Now()})
		if err != nil {
			return err
		}
		if n == 0 {
			return errors.WithStack(ErrMessageNotFound)
		}
	}
	return tx.Commit()
}
