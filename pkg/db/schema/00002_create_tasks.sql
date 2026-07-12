-- +goose Up
CREATE TABLE tasks (
  completed  INTEGER   NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  title      TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_tasks_created_at ON tasks(created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP TABLE IF EXISTS tasks;
