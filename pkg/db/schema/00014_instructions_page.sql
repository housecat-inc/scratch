-- +goose Up
INSERT INTO pages (description, id, pinned, title) VALUES
('Read workspace instructions.', 'agents', 1, 'AGENTS.md');

-- +goose Down
DELETE FROM pages WHERE id = 'agents';
