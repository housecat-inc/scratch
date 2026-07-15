-- +goose Up
ALTER TABLE threads ADD COLUMN trashed_at TIMESTAMP;

-- +goose Down
ALTER TABLE threads DROP COLUMN trashed_at;
