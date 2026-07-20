-- +goose Up
CREATE TABLE contact_notes (
  body       TEXT      NOT NULL,
  contact_id INTEGER   NOT NULL REFERENCES contacts(id) ON DELETE CASCADE,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT
);
CREATE INDEX idx_contact_notes_contact_id ON contact_notes(contact_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_contact_notes_contact_id;
DROP TABLE IF EXISTS contact_notes;
