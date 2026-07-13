-- +goose Up
CREATE TABLE sql_queries (
  created_at TIMESTAMP NOT NULL,
  id         TEXT      PRIMARY KEY,
  name       TEXT      NOT NULL UNIQUE,
  sql        TEXT      NOT NULL,
  updated_at TIMESTAMP NOT NULL
);

-- +goose Down
DROP TABLE sql_queries;
