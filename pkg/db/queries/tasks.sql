-- name: AddTask :one
INSERT INTO tasks (created_at, title, updated_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: AddTaskNote :one
INSERT INTO task_notes (body, created_at, task_id, updated_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: AddTaskSubitem :one
INSERT INTO task_subitems (created_at, task_id, title, updated_at)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: ClearCompletedTasks :execrows
DELETE FROM tasks WHERE completed = 1;

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = ?;

-- name: DeleteTaskSubitem :execrows
DELETE FROM task_subitems WHERE id = ?;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = ?;

-- name: GetTaskSubitem :one
SELECT * FROM task_subitems WHERE id = ?;

-- name: ListTaskNotes :many
SELECT * FROM task_notes WHERE task_id = ? ORDER BY created_at ASC, id ASC;

-- name: ListTasks :many
SELECT * FROM tasks ORDER BY created_at ASC;

-- name: ListTasksByArchived :many
SELECT * FROM tasks WHERE archived = ? ORDER BY created_at ASC;

-- name: ListTaskSubitems :many
SELECT * FROM task_subitems WHERE task_id = ? ORDER BY created_at ASC, id ASC;

-- name: SetTaskArchived :execrows
UPDATE tasks SET archived = ?, updated_at = ? WHERE id = ?;

-- name: SetTaskCompleted :execrows
UPDATE tasks SET completed = ?, updated_at = ? WHERE id = ?;

-- name: SetTaskStarred :execrows
UPDATE tasks SET starred = ?, updated_at = ? WHERE id = ?;

-- name: SetTaskSubitemCompleted :execrows
UPDATE task_subitems SET completed = ?, updated_at = ? WHERE id = ?;

-- name: UpdateTaskTitle :execrows
UPDATE tasks SET title = ?, updated_at = ? WHERE id = ?;
