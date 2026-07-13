-- name: DeleteSQLQuery :execrows
DELETE FROM sql_queries WHERE id = ?;

-- name: ListSQLQueries :many
SELECT * FROM sql_queries ORDER BY name ASC;

-- name: SaveSQLQuery :one
INSERT INTO sql_queries (created_at, id, name, sql, updated_at)
VALUES (?, ?, ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET sql = excluded.sql, updated_at = excluded.updated_at
RETURNING *;
