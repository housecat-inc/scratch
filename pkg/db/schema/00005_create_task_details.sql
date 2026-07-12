-- +goose Up
ALTER TABLE tasks ADD COLUMN archived INTEGER NOT NULL DEFAULT 0;

CREATE TABLE task_notes (
  body       TEXT      NOT NULL,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  task_id    INTEGER   NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  updated_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_task_notes_task_id_created_at ON task_notes(task_id, created_at);

CREATE TABLE task_subitems (
  completed  INTEGER   NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  task_id    INTEGER   NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
  title      TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
CREATE INDEX idx_task_subitems_task_id_created_at ON task_subitems(task_id, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_task_subitems_task_id_created_at;
DROP TABLE IF EXISTS task_subitems;
DROP INDEX IF EXISTS idx_task_notes_task_id_created_at;
DROP TABLE IF EXISTS task_notes;
CREATE TABLE tasks_unarchived (
  completed  INTEGER   NOT NULL DEFAULT 0,
  created_at TIMESTAMP NOT NULL,
  id         INTEGER   PRIMARY KEY AUTOINCREMENT,
  title      TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);
INSERT INTO tasks_unarchived (completed, created_at, id, title, updated_at)
SELECT completed, created_at, id, title, updated_at
FROM tasks;
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP TABLE tasks;
ALTER TABLE tasks_unarchived RENAME TO tasks;
CREATE INDEX idx_tasks_created_at ON tasks(created_at);
