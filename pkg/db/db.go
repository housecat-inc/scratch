package db

import (
	"context"
	"database/sql"
	"embed"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

//go:embed schema/*.sql
var schemaFS embed.FS

type DB struct {
	conn    *sql.DB
	queries *sqlite.Queries
}

func New(path string) (*DB, error) {
	if path != ":memory:" {
		if dir := filepath.Dir(path); dir != "" && dir != "." {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, errors.Wrapf(err, "mkdir %s", dir)
			}
		}
	}
	conn, err := sql.Open("sqlite", dsn(path))
	if err != nil {
		return nil, errors.Wrap(err, "open sqlite")
	}
	conn.SetMaxOpenConns(1)
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "migrate")
	}
	return &DB{conn: conn, queries: sqlite.New(conn)}, nil
}

func (d *DB) Close() error { return d.conn.Close() }

func migrate(conn *sql.DB) error {
	sub, err := fs.Sub(schemaFS, "schema")
	if err != nil {
		return errors.Wrap(err, "sub schema fs")
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, conn, sub)
	if err != nil {
		return errors.Wrap(err, "new goose provider")
	}
	if _, err := provider.Up(context.Background()); err != nil {
		return errors.Wrap(err, "goose up")
	}
	return nil
}

func dsn(path string) string {
	if path == ":memory:" {
		return path
	}
	v := url.Values{}
	v.Set("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_pragma", "foreign_keys(on)")
	v.Add("_time_format", "sqlite")
	return "file:" + path + "?" + v.Encode()
}
