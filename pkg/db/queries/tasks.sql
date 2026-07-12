-- name: AddTask :one
INSERT INTO tasks (created_at, title, updated_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: ClearCompletedTasks :execrows
DELETE FROM tasks WHERE completed = 1;

-- name: DeleteTask :execrows
DELETE FROM tasks WHERE id = ?;

-- name: GetTask :one
SELECT * FROM tasks WHERE id = ?;

-- name: ListTasks :many
SELECT * FROM tasks ORDER BY created_at ASC;

-- name: SetTaskCompleted :execrows
UPDATE tasks SET completed = ?, updated_at = ? WHERE id = ?;

-- name: UpdateTaskTitle :execrows
UPDATE tasks SET title = ?, updated_at = ? WHERE id = ?;
