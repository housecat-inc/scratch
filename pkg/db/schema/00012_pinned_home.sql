-- +goose Up
ALTER TABLE pages ADD COLUMN home BOOLEAN NOT NULL DEFAULT 0;
INSERT INTO pages (description, home, id, title) VALUES
('Get to know Scratch and start creating.', 1, 'getting-started', 'Getting Started');

-- +goose Down
DELETE FROM pages WHERE id = 'getting-started';
ALTER TABLE pages DROP COLUMN home;
