-- +goose Up
CREATE TABLE threads (
  anchor_json TEXT      NOT NULL DEFAULT '{}',
  created_at  TIMESTAMP NOT NULL,
  id          INTEGER   PRIMARY KEY AUTOINCREMENT,
  kind        TEXT      NOT NULL,
  state       TEXT      NOT NULL DEFAULT 'open',
  title       TEXT      NOT NULL DEFAULT '',
  updated_at  TIMESTAMP NOT NULL
);
CREATE INDEX idx_threads_kind_updated_at ON threads(kind, updated_at);

CREATE TABLE messages (
  author       TEXT      NOT NULL,
  body         TEXT      NOT NULL,
  completed_at TIMESTAMP,
  created_at   TIMESTAMP NOT NULL,
  id           INTEGER   PRIMARY KEY AUTOINCREMENT,
  meta_json    TEXT      NOT NULL DEFAULT '{}',
  parent_id    INTEGER   REFERENCES messages(id),
  read_at      TIMESTAMP,
  role         TEXT      NOT NULL,
  seq          INTEGER   NOT NULL,
  status       TEXT      NOT NULL DEFAULT 'complete',
  thread_id    INTEGER   NOT NULL REFERENCES threads(id),
  updated_at   TIMESTAMP NOT NULL
);
CREATE UNIQUE INDEX idx_messages_thread_id_seq ON messages(thread_id, seq);

CREATE TABLE message_events (
  created_at TIMESTAMP NOT NULL,
  data_json  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   NOT NULL REFERENCES messages(id),
  seq        INTEGER   NOT NULL,
  type       TEXT      NOT NULL
);
CREATE UNIQUE INDEX idx_message_events_message_id_seq ON message_events(message_id, seq);

CREATE TABLE attachments (
  created_at TIMESTAMP NOT NULL,
  file_path  TEXT      NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  message_id INTEGER   NOT NULL REFERENCES messages(id),
  mime_type  TEXT      NOT NULL,
  name       TEXT      NOT NULL,
  size       INTEGER   NOT NULL
);
CREATE INDEX idx_attachments_message_id ON attachments(message_id);

-- +goose Down
DROP INDEX IF EXISTS idx_attachments_message_id;
DROP TABLE IF EXISTS attachments;
DROP INDEX IF EXISTS idx_message_events_message_id_seq;
DROP TABLE IF EXISTS message_events;
DROP INDEX IF EXISTS idx_messages_thread_id_seq;
DROP TABLE IF EXISTS messages;
DROP INDEX IF EXISTS idx_threads_kind_updated_at;
DROP TABLE IF EXISTS threads;
