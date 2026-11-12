-- +goose Up
CREATE TABLE attachments_new (
  created_at TIMESTAMP NOT NULL,
  file_path  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   REFERENCES messages(id),
  mime_type  TEXT      NOT NULL,
  name       TEXT      NOT NULL,
  size       INTEGER   NOT NULL,
  thread_id  INTEGER   REFERENCES threads(id)
);
INSERT INTO attachments_new (created_at, file_path, id, message_id, mime_type, name, size, thread_id)
SELECT created_at, file_path, id, message_id, mime_type, name, size, thread_id FROM attachments;
DROP TABLE attachments;
ALTER TABLE attachments_new RENAME TO attachments;
CREATE INDEX idx_attachments_message_id ON attachments(message_id);
CREATE INDEX idx_attachments_thread_id ON attachments(thread_id);

-- +goose Down
DELETE FROM attachments WHERE thread_id IS NULL;
CREATE TABLE attachments_old (
  created_at TIMESTAMP NOT NULL,
  file_path  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   REFERENCES messages(id),
  mime_type  TEXT      NOT NULL,
  name       TEXT      NOT NULL,
  size       INTEGER   NOT NULL,
  thread_id  INTEGER   NOT NULL REFERENCES threads(id)
);
INSERT INTO attachments_old (created_at, file_path, id, message_id, mime_type, name, size, thread_id)
SELECT created_at, file_path, id, message_id, mime_type, name, size, thread_id FROM attachments;
DROP TABLE attachments;
ALTER TABLE attachments_old RENAME TO attachments;
CREATE INDEX idx_attachments_message_id ON attachments(message_id);
CREATE INDEX idx_attachments_thread_id ON attachments(thread_id);
