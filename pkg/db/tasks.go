package db

import (
	"context"
	"database/sql"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

type Task struct {
	Archived    bool
	Completed   bool
	CreatedAt   ts.Timestamp
	Description string
	ID          int64
	Starred     bool
	Title       string
	UpdatedAt   ts.Timestamp
}

type TaskNote struct {
	Body      string
	CreatedAt ts.Timestamp
	ID        int64
	TaskID    int64
	UpdatedAt ts.Timestamp
}

type TaskStore interface {
	AddTask(title string) (Task, error)
	AddTaskNote(taskID int64, body string) (TaskNote, error)
	AddTaskSubitem(taskID int64, title string) (TaskSubitem, error)
	ClearCompletedTasks() (int, error)
	DeleteTask(id int64) error
	DeleteTaskSubitem(id int64) error
	GetTask(id int64) (Task, error)
	ListTaskNotes(taskID int64) ([]TaskNote, error)
	ListTaskSubitems(taskID int64) ([]TaskSubitem, error)
	ListTasks() ([]Task, error)
	ListTasksByArchived(archived bool) ([]Task, error)
	SetTaskArchived(id int64, archived bool) (Task, error)
	SetTaskCompleted(id int64, completed bool) (Task, error)
	SetTaskStarred(id int64, starred bool) (Task, error)
	SetTaskSubitemCompleted(id int64, completed bool) (TaskSubitem, error)
	UpdateTaskDescription(id int64, description string) (Task, error)
	UpdateTaskTitle(id int64, title string) (Task, error)
}

type TaskSubitem struct {
	Completed bool
	CreatedAt ts.Timestamp
	ID        int64
	TaskID    int64
	Title     string
	UpdatedAt ts.Timestamp
}

var ErrTaskNotFound = errors.New("task not found")
var ErrTaskSubitemNotFound = errors.New("task subitem not found")

func IsTaskNotFound(err error) bool        { return errors.Is(err, ErrTaskNotFound) }
func IsTaskSubitemNotFound(err error) bool { return errors.Is(err, ErrTaskSubitemNotFound) }

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

func (d *DB) AddTaskNote(taskID int64, body string) (TaskNote, error) {
	now := ts.Now()
	row, err := d.queries.AddTaskNote(context.Background(), sqlite.AddTaskNoteParams{
		Body:      body,
		CreatedAt: now,
		TaskID:    taskID,
		UpdatedAt: now,
	})
	if err != nil {
		return TaskNote{}, errors.Wrap(err, "insert task note")
	}
	return fromSqliteTaskNote(row), nil
}

func (d *DB) AddTaskSubitem(taskID int64, title string) (TaskSubitem, error) {
	now := ts.Now()
	row, err := d.queries.AddTaskSubitem(context.Background(), sqlite.AddTaskSubitemParams{
		CreatedAt: now,
		TaskID:    taskID,
		Title:     title,
		UpdatedAt: now,
	})
	if err != nil {
		return TaskSubitem{}, errors.Wrap(err, "insert task subitem")
	}
	return fromSqliteTaskSubitem(row), nil
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

func (d *DB) DeleteTaskSubitem(id int64) error {
	n, err := d.queries.DeleteTaskSubitem(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "delete task subitem")
	}
	if n == 0 {
		return ErrTaskSubitemNotFound
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

func (d *DB) ListTaskNotes(taskID int64) ([]TaskNote, error) {
	rows, err := d.queries.ListTaskNotes(context.Background(), taskID)
	if err != nil {
		return nil, errors.Wrap(err, "list task notes")
	}
	out := make([]TaskNote, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteTaskNote(r))
	}
	return out, nil
}

func (d *DB) ListTaskSubitems(taskID int64) ([]TaskSubitem, error) {
	rows, err := d.queries.ListTaskSubitems(context.Background(), taskID)
	if err != nil {
		return nil, errors.Wrap(err, "list task subitems")
	}
	out := make([]TaskSubitem, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteTaskSubitem(r))
	}
	return out, nil
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

func (d *DB) ListTasksByArchived(archived bool) ([]Task, error) {
	flag := int64(0)
	if archived {
		flag = 1
	}
	rows, err := d.queries.ListTasksByArchived(context.Background(), flag)
	if err != nil {
		return nil, errors.Wrap(err, "list tasks by archived")
	}
	out := make([]Task, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteTask(r))
	}
	return out, nil
}

func (d *DB) SetTaskArchived(id int64, archived bool) (Task, error) {
	flag := int64(0)
	if archived {
		flag = 1
	}
	n, err := d.queries.SetTaskArchived(context.Background(), sqlite.SetTaskArchivedParams{
		Archived:  flag,
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "set archived")
	}
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}
	return d.GetTask(id)
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

func (d *DB) SetTaskStarred(id int64, starred bool) (Task, error) {
	flag := int64(0)
	if starred {
		flag = 1
	}
	n, err := d.queries.SetTaskStarred(context.Background(), sqlite.SetTaskStarredParams{
		ID:        id,
		Starred:   flag,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "set starred")
	}
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}
	return d.GetTask(id)
}

func (d *DB) SetTaskSubitemCompleted(id int64, completed bool) (TaskSubitem, error) {
	flag := int64(0)
	if completed {
		flag = 1
	}
	n, err := d.queries.SetTaskSubitemCompleted(context.Background(), sqlite.SetTaskSubitemCompletedParams{
		Completed: flag,
		ID:        id,
		UpdatedAt: ts.Now(),
	})
	if err != nil {
		return TaskSubitem{}, errors.Wrap(err, "set subitem completed")
	}
	if n == 0 {
		return TaskSubitem{}, ErrTaskSubitemNotFound
	}
	row, err := d.queries.GetTaskSubitem(context.Background(), id)
	if errors.Is(err, sql.ErrNoRows) {
		return TaskSubitem{}, ErrTaskSubitemNotFound
	}
	if err != nil {
		return TaskSubitem{}, errors.Wrap(err, "get task subitem")
	}
	return fromSqliteTaskSubitem(row), nil
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

func (d *DB) UpdateTaskDescription(id int64, description string) (Task, error) {
	n, err := d.queries.UpdateTaskDescription(context.Background(), sqlite.UpdateTaskDescriptionParams{
		Description: description,
		ID:          id,
		UpdatedAt:   ts.Now(),
	})
	if err != nil {
		return Task{}, errors.Wrap(err, "update description")
	}
	if n == 0 {
		return Task{}, ErrTaskNotFound
	}
	return d.GetTask(id)
}

func fromSqliteTask(t sqlite.Task) Task {
	return Task{
		Archived:    t.Archived != 0,
		Completed:   t.Completed != 0,
		CreatedAt:   t.CreatedAt,
		Description: t.Description,
		ID:          t.ID,
		Starred:     t.Starred != 0,
		Title:       t.Title,
		UpdatedAt:   t.UpdatedAt,
	}
}

func fromSqliteTaskNote(n sqlite.TaskNote) TaskNote {
	return TaskNote{
		Body:      n.Body,
		CreatedAt: n.CreatedAt,
		ID:        n.ID,
		TaskID:    n.TaskID,
		UpdatedAt: n.UpdatedAt,
	}
}

func fromSqliteTaskSubitem(s sqlite.TaskSubitem) TaskSubitem {
	return TaskSubitem{
		Completed: s.Completed != 0,
		CreatedAt: s.CreatedAt,
		ID:        s.ID,
		TaskID:    s.TaskID,
		Title:     s.Title,
		UpdatedAt: s.UpdatedAt,
	}
}
