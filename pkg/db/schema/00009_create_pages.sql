-- +goose Up
CREATE TABLE pages (
    description TEXT NOT NULL,
    id TEXT PRIMARY KEY,
    pinned BOOLEAN NOT NULL DEFAULT 1,
    thread_id INTEGER REFERENCES threads(id) ON DELETE SET NULL,
    title TEXT NOT NULL
);
INSERT INTO pages (description, id, title) VALUES
('Explore a page, pin it as home, or edit it with your agent.', 'example', 'Example Page');

-- +goose Down
DROP TABLE pages;
