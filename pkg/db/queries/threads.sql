-- name: AddThread :one
INSERT INTO threads (anchor_json, created_at, kind, state, title, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetThread :one
SELECT * FROM threads WHERE id = ?;

-- name: ListThreads :many
SELECT * FROM threads WHERE kind = ? ORDER BY updated_at DESC, id DESC;

-- name: SetThreadAnchor :execrows
UPDATE threads SET anchor_json = ?, updated_at = ? WHERE id = ?;

-- name: SetThreadState :execrows
UPDATE threads SET state = ?, updated_at = ? WHERE id = ?;

-- name: SetThreadTitle :execrows
UPDATE threads SET title = ?, updated_at = ? WHERE id = ?;

-- name: TouchThread :execrows
UPDATE threads SET updated_at = ? WHERE id = ?;
