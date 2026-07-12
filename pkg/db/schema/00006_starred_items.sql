-- +goose Up
ALTER TABLE tasks ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;
ALTER TABLE threads ADD COLUMN starred INTEGER NOT NULL DEFAULT 0;

-- +goose Down
CREATE TABLE tasks_unstarred (
  archived   INTEGER   NOT NULL DEFAULT 0,
  completed  INTEGER   NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  title      TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
INSERT INTO tasks_unstarred (archived, completed, created_at, id, title, updated_at)
SELECT archived, completed, created_at, id, title, updated_at
FROM tasks;
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP TABLE tasks;
ALTER TABLE tasks_unstarred RENAME TO tasks;
CREATE INDEX idx_tasks_created_at ON tasks(created_at);

CREATE TABLE threads_unstarred (
  anchor_json TEXT      NOT NULL DEFAULT '{}',
  created_at  TIMESTAMP NOT NULL,
  id          INTEGER   PRIMARY KEY AUTOINCREMENT,
  kind        TEXT      NOT NULL,
  state       TEXT      NOT NULL DEFAULT 'open',
  title       TEXT      NOT NULL DEFAULT '',
  updated_at  TIMESTAMP NOT NULL
);
INSERT INTO threads_unstarred (anchor_json, created_at, id, kind, state, title, updated_at)
SELECT anchor_json, created_at, id, kind, state, title, updated_at
FROM threads;
DROP INDEX IF EXISTS idx_threads_kind_updated_at;
DROP TABLE threads;
ALTER TABLE threads_unstarred RENAME TO threads;
CREATE INDEX idx_threads_kind_updated_at ON threads(kind, updated_at);
