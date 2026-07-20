-- name: AddContact :one
INSERT INTO contacts (company, created_at, job_title, name, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: AddContactNote :one
INSERT INTO contact_notes (body, contact_id, created_at)
VALUES (?, ?, ?)
RETURNING *;

-- name: AddEmail :one
INSERT INTO emails (contact_id, created_at, email, is_primary, updated_at)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: CountContactNotes :one
SELECT COUNT(*) FROM contact_notes WHERE contact_id = ?;

-- name: ListContactNotes :many
SELECT * FROM contact_notes WHERE contact_id = ? ORDER BY created_at DESC, id DESC;

-- name: CountContacts :one
SELECT COUNT(*) FROM contacts;

-- name: DeleteContact :execrows
DELETE FROM contacts WHERE id = ?;

-- name: DeleteEmail :execrows
DELETE FROM emails WHERE id = ?;

-- name: GetContact :one
SELECT * FROM contacts WHERE id = ?;

-- name: ListContactEmails :many
SELECT * FROM emails WHERE contact_id = ? ORDER BY is_primary DESC, id ASC;

-- name: ListContactsPage :many
SELECT
  c.*,
  COALESCE((SELECT e.email FROM emails e WHERE e.contact_id = c.id ORDER BY e.is_primary DESC, e.id ASC LIMIT 1), '') AS primary_email
FROM contacts c
ORDER BY c.name ASC, c.id ASC
LIMIT ? OFFSET ?;

-- name: SearchContacts :many
SELECT DISTINCT c.*,
  COALESCE((SELECT e.email FROM emails e WHERE e.contact_id = c.id ORDER BY e.is_primary DESC, e.id ASC LIMIT 1), '') AS primary_email
FROM contacts c
LEFT JOIN emails e ON e.contact_id = c.id
WHERE c.name LIKE ? OR e.email LIKE ?
ORDER BY c.name ASC, c.id ASC
LIMIT ?;

-- name: UpdateContact :execrows
UPDATE contacts SET company = ?, job_title = ?, name = ?, updated_at = ? WHERE id = ?;
