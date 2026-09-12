-- +goose Up
INSERT INTO pages (description, id, pinned, title) VALUES
('Browse your conversations.', 'chats', 1, 'Chats'),
('Browse repositories and review changes.', 'code', 0, 'Code'),
('Browse and edit workspace files.', 'files', 0, 'Files'),
('See your latest work.', 'inbox', 0, 'Inbox'),
('Browse all your pages.', 'pages', 0, 'Pages'),
('Manage agent sessions.', 'sessions', 0, 'Sessions'),
('Connect and configure your agents.', 'setup', 1, 'Setup'),
('Find your starred items.', 'starred', 0, 'Starred'),
('Track your tasks.', 'tasks', 1, 'Tasks'),
('Manage scheduled and durable jobs.', 'workflows', 1, 'Workflows');

-- +goose Down
DELETE FROM pages WHERE id IN ( 'chats', 'code', 'files', 'inbox', 'pages', 'sessions', 'setup', 'starred', 'tasks', 'workflows');
