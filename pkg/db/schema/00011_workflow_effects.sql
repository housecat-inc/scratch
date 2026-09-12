-- +goose Up
CREATE TABLE workflow_effects (
  effect_key TEXT PRIMARY KEY,
  result_json TEXT NOT NULL DEFAULT ''
);

-- +goose Down
DROP TABLE workflow_effects;
