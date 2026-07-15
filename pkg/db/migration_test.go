package db

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/pressly/goose/v3"
	"github.com/stretchr/testify/require"
)

func TestMigrateUpgradesExistingDatabase(t *testing.T) {
	r := require.New(t)

	dir := t.TempDir()
	old := filepath.Join(dir, "old")
	r.NoError(os.MkdirAll(old, 0o755))
	entries, err := os.ReadDir("schema")
	r.NoError(err)
	for _, entry := range entries {
		if entry.Name() >= "00010" {
			continue
		}
		data, err := os.ReadFile(filepath.Join("schema", entry.Name()))
		r.NoError(err)
		r.NoError(os.WriteFile(filepath.Join(old, entry.Name()), data, 0o644))
	}

	path := filepath.Join(dir, "scratch.db")
	conn, err := sql.Open("sqlite", dsn(path))
	r.NoError(err)
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, os.DirFS(old))
	r.NoError(err)
	_, err = provider.Up(context.Background())
	r.NoError(err)
	_, err = conn.ExecContext(context.Background(),
		`INSERT INTO threads (anchor_json, created_at, kind, state, title, updated_at) VALUES ('{}', '2026-01-01T00:00:00Z', 'chat', 'open', 'Legacy', '2026-01-01T00:00:00Z')`)
	r.NoError(err)
	r.NoError(conn.Close())

	d, err := New(path)
	r.NoError(err)
	defer d.Close()

	threads, err := d.ListThreads(ThreadKindChat)
	r.NoError(err)
	r.Len(threads, 1)
	r.NoError(d.TrashThread(threads[0].ID))

	threads, err = d.ListThreads(ThreadKindChat)
	r.NoError(err)
	r.Empty(threads)
}
