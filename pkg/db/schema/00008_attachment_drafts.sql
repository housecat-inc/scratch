-- +goose Up
DROP TABLE attachments;

CREATE TABLE attachments (
  created_at TIMESTAMP NOT NULL,
  file_path  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   REFERENCES messages(id),
  mime_type  TEXT      NOT NULL,
  name       TEXT      NOT NULL,
  size       INTEGER   NOT NULL,
  thread_id  INTEGER   NOT NULL REFERENCES threads(id)
);

CREATE INDEX idx_attachments_message_id ON attachments(message_id);
CREATE INDEX idx_attachments_thread_id ON attachments(thread_id);

-- +goose Down
DROP TABLE attachments;

CREATE TABLE attachments (
  created_at TIMESTAMP NOT NULL,
  file_path  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   NOT NULL REFERENCES messages(id),
  mime_type  TEXT      NOT NULL,
  name       TEXT      NOT NULL,
  size       INTEGER   NOT NULL
);
