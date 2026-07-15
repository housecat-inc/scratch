-- name: AddThread :one
INSERT INTO threads (anchor_json, created_at, kind, state, title, updated_at)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteThread :execrows
DELETE FROM threads WHERE id = ?;

-- name: DeleteThreadAttachments :exec
DELETE FROM attachments WHERE thread_id = ?;

-- name: DeleteThreadMessageEvents :exec
DELETE FROM message_events
WHERE message_id IN (SELECT id FROM messages WHERE thread_id = ?);

-- name: DeleteThreadMessages :exec
DELETE FROM messages WHERE thread_id = ?;

-- name: GetThread :one
SELECT * FROM threads WHERE id = ?;

-- name: ListThreads :many
SELECT * FROM threads WHERE kind = ? AND trashed_at IS NULL ORDER BY updated_at DESC, id DESC;

-- name: SetThreadAnchor :execrows
UPDATE threads SET anchor_json = ?, updated_at = ? WHERE id = ?;

-- name: SetThreadState :execrows
UPDATE threads SET state = ?, updated_at = ? WHERE id = ?;

-- name: SetThreadStarred :execrows
UPDATE threads SET starred = ?, updated_at = ? WHERE id = ?;

-- name: SetThreadTitle :execrows
UPDATE threads SET title = ?, updated_at = ? WHERE id = ?;

-- name: TouchThread :execrows
UPDATE threads SET updated_at = ? WHERE id = ?;

-- name: TrashThread :execrows
UPDATE threads SET starred = 0, trashed_at = ?, updated_at = ? WHERE id = ?;
