-- name: AddComment :one
INSERT INTO comments (body, created, id, line, path, resolved, side, slug, updated)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: DeleteComment :execrows
DELETE FROM comments WHERE id = ? AND slug = ?;

-- name: GetComment :one
SELECT * FROM comments WHERE id = ? AND slug = ?;

-- name: ListComments :many
SELECT * FROM comments WHERE slug = ? ORDER BY created ASC;

-- name: SetCommentResolved :execrows
UPDATE comments SET resolved = ?, updated = ? WHERE id = ? AND slug = ?;

-- name: UpdateCommentBody :execrows
UPDATE comments SET body = ?, updated = ? WHERE id = ? AND slug = ?;
