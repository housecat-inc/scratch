-- +goose Up
CREATE TABLE comments_rfc3339 (
  body          TEXT      NOT NULL,
  branch        TEXT      NOT NULL,
  created_at    TIMESTAMP NOT NULL,
  id            TEXT      PRIMARY KEY,
  line          INTEGER   NOT NULL,
  path          TEXT      NOT NULL,
  resolved      INTEGER   NOT NULL DEFAULT 0,
  resolved_body TEXT      NOT NULL DEFAULT '',
  side          TEXT      NOT NULL,
  slug          TEXT      NOT NULL,
  updated_at    TIMESTAMP NOT NULL
);
INSERT INTO comments_rfc3339 (body, branch, created_at, id, line, path, resolved, resolved_body, side, slug, updated_at)
SELECT body, branch, strftime('%Y-%m-%dT%H:%M:%SZ', created_at), id, line, path, resolved, resolved_body, side, slug, strftime('%Y-%m-%dT%H:%M:%SZ', updated_at)
FROM comments;
DROP INDEX IF EXISTS idx_comments_slug_branch;
DROP TABLE comments;
ALTER TABLE comments_rfc3339 RENAME TO comments;
CREATE INDEX idx_comments_slug_branch ON comments(slug, branch);

-- +goose Down
CREATE TABLE comments_datetime (
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
INSERT INTO comments_datetime (body, branch, created_at, id, line, path, resolved, resolved_body, side, slug, updated_at)
SELECT body, branch, created_at, id, line, path, resolved, resolved_body, side, slug, updated_at
FROM comments;
DROP INDEX IF EXISTS idx_comments_slug_branch;
DROP TABLE comments;
ALTER TABLE comments_datetime RENAME TO comments;
CREATE INDEX idx_comments_slug_branch ON comments(slug, branch);
