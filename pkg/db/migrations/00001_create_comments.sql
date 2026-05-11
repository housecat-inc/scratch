-- +goose Up
CREATE TABLE comments (
  body     TEXT    NOT NULL,
  created  INTEGER NOT NULL,
  id       TEXT    PRIMARY KEY,
  line     INTEGER NOT NULL,
  path     TEXT    NOT NULL,
  resolved INTEGER NOT NULL DEFAULT 0,
  side     TEXT    NOT NULL,
  slug     TEXT    NOT NULL,
  updated  INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX idx_comments_slug ON comments(slug);

-- +goose Down
DROP INDEX IF EXISTS idx_comments_slug;
DROP TABLE IF EXISTS comments;
