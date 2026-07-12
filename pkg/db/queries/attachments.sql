-- name: AddAttachment :one
INSERT INTO attachments (created_at, file_path, mime_type, name, size, thread_id)
VALUES (?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteDraftAttachment :one
DELETE FROM attachments WHERE id = ? AND message_id IS NULL
RETURNING *;

-- name: GetAttachment :one
SELECT * FROM attachments WHERE id = ?;

-- name: LinkAttachments :exec
UPDATE attachments SET message_id = @message_id
WHERE thread_id = @thread_id AND message_id IS NULL AND id IN (sqlc.slice('ids'));

-- name: ListMessageAttachments :many
SELECT * FROM attachments WHERE message_id = ? ORDER BY id ASC;

-- name: ListThreadAttachments :many
SELECT * FROM attachments WHERE thread_id = ? AND message_id IS NOT NULL ORDER BY id ASC;
