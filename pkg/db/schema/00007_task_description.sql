-- +goose Up
ALTER TABLE tasks ADD COLUMN description TEXT NOT NULL DEFAULT '';

-- +goose Down
CREATE TABLE tasks_without_description (
  archived   INTEGER   NOT NULL DEFAULT 0,
  completed  INTEGER   NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  starred    INTEGER   NOT NULL DEFAULT 0,
  title      TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
INSERT INTO tasks_without_description (archived, completed, created_at, id, starred, title, updated_at)
SELECT archived, completed, created_at, id, starred, title, updated_at
FROM tasks;
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP TABLE tasks;
ALTER TABLE tasks_without_description RENAME TO tasks;
CREATE INDEX idx_tasks_created_at ON tasks(created_at);
