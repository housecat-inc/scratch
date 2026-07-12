package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

type Task struct {
	Completed bool
	CreatedAt ts.Timestamp
	ID        int64
	Title     string
	UpdatedAt ts.Timestamp
}

type TaskStore interface {
	AddTask(title string) (Task, error)
	ClearCompletedTasks() (int, error)
	DeleteTask(id int64) error
	GetTask(id int64) (Task, error)
	ListTasks() ([]Task, error)
	SetTaskCompleted(id int64, completed bool) (Task, error)
	UpdateTaskTitle(id int64, title string) (Task, error)
}

var ErrTaskNotFound = errors.New("task not found")

func IsTaskNotFound(err error) bool { return errors.Is(err, ErrTaskNotFound) }

func (d *DB) AddTask(title string) (Task, error) {
	now := ts.Now()
	row, err := d.queries.AddTask(context.Background(), sqlite.AddTaskParams{
		CreatedAt: now,
		Title:     title,
		UpdatedAt: now,
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "insert task")
	}
	return fromSqliteTask(row), nil
}

func (d *DB) ClearCompletedTasks() (int, error) {
	n, err := d.queries.ClearCompletedTasks(context.Background())
	if err != nil {
		return 0, errors.Wrap(err, "clear completed tasks")
	}
	return int(n), nil
}

func (d *DB) DeleteTask(id int64) error {
	n, err := d.queries.DeleteTask(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "delete task")
	}
	if n == 0 {
		return ErrTaskNotFound
	}
	return nil
}

func (d *DB) GetTask(id int64) (Task, error) {
	row, err := d.queries.GetTask(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return Task{}, ErrTaskNotFound
	}
	if err != nil {
		return Task{}, errors.Wrap(err, "get task")
	}
	return fromSqliteTask(row), nil
}

func (d *DB) ListTasks() ([]Task, error) {
	rows, err := d.queries.ListTasks(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "list tasks")
	}
	out := make([]Task, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteTask(r))
	}
	return out, nil
}

func (d *DB) SetTaskCompleted(id int64, completed bool) (Task, error) {
	flag := int64(0)
	if completed {
		flag = 1
	}
	n, err := d.queries.SetTaskCompleted(context.Background(), sqlite.SetTaskCompletedParams{
		Completed: flag,
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "set completed")
	}
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}
	return d.GetTask(id)
}

func (d *DB) UpdateTaskTitle(id int64, title string) (Task, error) {
	n, err := d.queries.UpdateTaskTitle(context.Background(), sqlite.UpdateTaskTitleParams{
		ID:        id,
		Title:     title,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "update title")
	}
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}
	return d.GetTask(id)
}

func fromSqliteTask(t sqlite.Task) Task {
	return Task{
		Completed: t.Completed != 0,
		CreatedAt: t.CreatedAt,
		ID:        t.ID,
		Title:     t.Title,
		UpdatedAt: t.UpdatedAt,
	}
}
