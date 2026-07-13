package db

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db/internal/sqlite"
	"github.com/housecat-inc/scratch/pkg/ts"
)

type SQLQuery struct {
	CreatedAt time.Time
	ID        string
	Name      string
	SQL       string
	UpdatedAt time.Time
}

type SQLQueryStore interface {
	DeleteSQLQuery(id string) error
	ListSQLQueries() ([]SQLQuery, error)
	SaveSQLQuery(name, sql string) (SQLQuery, error)
}

var ErrSQLQueryNotFound = errors.New("sql query not found")

type NopSQLQueryStore struct{}

func (NopSQLQueryStore) DeleteSQLQuery(string) error         { return ErrSQLQueryNotFound }
func (NopSQLQueryStore) ListSQLQueries() ([]SQLQuery, error) { return nil, nil }
func (NopSQLQueryStore) SaveSQLQuery(string, string) (SQLQuery, error) {
	return SQLQuery{}, errors.New("saved queries disabled")
}

func (d *DB) DeleteSQLQuery(id string) error {
	n, err := d.queries.DeleteSQLQuery(context.Background(), id)
	if err != nil {
		return errors.Wrap(err, "delete sql query")
	}
	if n == 0 {
		return ErrSQLQueryNotFound
	}
	return nil
}

func (d *DB) ListSQLQueries() ([]SQLQuery, error) {
	rows, err := d.queries.ListSQLQueries(context.Background())
	if err != nil {
		return nil, errors.Wrap(err, "list sql queries")
	}
	out := make([]SQLQuery, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromSqliteSQLQuery(r))
	}
	return out, nil
}

func (d *DB) SaveSQLQuery(name, sql string) (SQLQuery, error) {
	now := ts.Now()
	row, err := d.queries.SaveSQLQuery(context.Background(), sqlite.SaveSQLQueryParams{
		CreatedAt: now,
		ID:        newSQLQueryID(),
		Name:      name,
		Sql:       sql,
		UpdatedAt: now,
	})
	if err != nil {
		return SQLQuery{}, errors.Wrap(err, "save sql query")
	}
	return fromSqliteSQLQuery(row), nil
}

func fromSqliteSQLQuery(q sqlite.SqlQuery) SQLQuery {
	return SQLQuery{
		CreatedAt: q.CreatedAt.Time,
		ID:        q.ID,
		Name:      q.Name,
		SQL:       q.Sql,
		UpdatedAt: q.UpdatedAt.Time,
	}
}

func newSQLQueryID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	return hex.EncodeToString(b[:])
}
