-- name: AddComment :one
INSERT INTO comments (body, branch, created_at, id, line, path, side, slug, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteComment :execrows
DELETE FROM comments WHERE id = ? AND slug = ?;

-- name: GetComment :one
SELECT * FROM comments WHERE id = ? AND slug = ?;

-- name: ListComments :many
SELECT * FROM comments WHERE slug = ? AND branch = ? ORDER BY created_at ASC;

-- name: SetCommentResolved :execrows
UPDATE comments SET resolved = ?, resolved_body = ?, updated_at = ? WHERE id = ? AND slug = ?;

-- name: UpdateCommentBody :execrows
UPDATE comments SET body = ?, updated_at = ? WHERE id = ? AND slug = ?;
