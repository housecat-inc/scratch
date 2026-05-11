-- +goose Up
CREATE TABLE comments (
  body          TEXT     NOT NULL,
  branch        TEXT     NOT NULL,
  created_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  id            TEXT     PRIMARY KEY,
  line          INTEGER  NOT NULL,
  path          TEXT     NOT NULL,
  resolved      INTEGER  NOT NULL DEFAULT 0,
  resolved_body TEXT     NOT NULL DEFAULT '',
  side          TEXT     NOT NULL,
  slug          TEXT     NOT NULL,
  updated_at    DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_comments_slug_branch ON comments(slug, branch);

-- +goose Down
DROP INDEX IF EXISTS idx_comments_slug_branch;
DROP TABLE IF EXISTS comments;
