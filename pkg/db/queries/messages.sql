-- name: AddMessage :one
INSERT INTO messages (author, body, created_at, meta_json, parent_id, role, seq, status, thread_id, updated_at)
VALUES (@author, @body, @created_at, @meta_json, @parent_id, @role,
  (SELECT COALESCE(MAX(m.seq), 0) + 1 FROM messages m WHERE m.thread_id = @thread_id),
  @status, @thread_id, @updated_at)
RETURNING *;

-- name: AddMessageEvent :one
INSERT INTO message_events (created_at, data_json, message_id, seq, type)
VALUES (@created_at, @data_json, @message_id,
  (SELECT COALESCE(MAX(e.seq), 0) + 1 FROM message_events e WHERE e.message_id = @message_id),
  @type)
RETURNING *;

-- name: AppendMessageBody :execrows
UPDATE messages SET body = body || ?, updated_at = ? WHERE id = ?;

-- name: FinishMessage :execrows
UPDATE messages SET completed_at = ?, status = ?, updated_at = ? WHERE id = ?;

-- name: GetMessage :one
SELECT * FROM messages WHERE id = ?;

-- name: ListMessageEvents :many
SELECT * FROM message_events WHERE message_id = ? AND seq > ? ORDER BY seq ASC;

-- name: ListMessagesByStatus :many
SELECT * FROM messages WHERE status = ? ORDER BY id ASC;

-- name: ListThreadMessageEvents :many
SELECT e.* FROM message_events e
JOIN messages m ON m.id = e.message_id
WHERE m.thread_id = ?
ORDER BY e.message_id ASC, e.seq ASC;

-- name: ListThreadMessages :many
SELECT * FROM messages WHERE thread_id = ? ORDER BY seq ASC;
