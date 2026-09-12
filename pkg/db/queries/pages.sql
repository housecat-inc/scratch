-- name: ListPages :many
SELECT * FROM pages ORDER BY title, id;

-- name: GetPage :one
SELECT * FROM pages WHERE id = ?;

-- name: GetPageByThread :one
SELECT * FROM pages WHERE thread_id = ?;

-- name: SetPagePinned :execrows
UPDATE pages SET pinned = ? WHERE id = ?;

-- name: SetPageThread :execrows
UPDATE pages SET thread_id = ? WHERE id = ?;

-- name: SetPageHome :execrows
UPDATE pages SET home = (pages.id = sqlc.arg(id))
WHERE EXISTS (SELECT 1 FROM pages AS candidate WHERE candidate.id = sqlc.arg(id));
