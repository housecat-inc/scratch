package workflow

import (
	"context"
	"database/sql"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	_ "modernc.org/sqlite"
)

type Workflows struct {
	conn *sql.DB
	ctx  dbos.DBOSContext
}

func New(path string) (*Workflows, error) {
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
	ctx, err := dbos.NewDBOSContext(context.Background(), dbos.Config{
		AppName:        "scratch",
		SqliteSystemDB: conn,
	})
	if err != nil {
		conn.Close()
		return nil, errors.Wrap(err, "new dbos context")
	}
	dbos.RegisterWorkflow(ctx, Greet)
	return &Workflows{conn: conn, ctx: ctx}, nil
}

func (w *Workflows) Launch() error {
	return errors.Wrap(dbos.Launch(w.ctx), "launch dbos")
}

func (w *Workflows) Close() error {
	dbos.Shutdown(w.ctx, 5*time.Second)
	return w.conn.Close()
}

func (w *Workflows) Greet(name string) (string, error) {
	handle, err := dbos.RunWorkflow(w.ctx, Greet, name)
	if err != nil {
		return "", errors.Wrap(err, "run greet workflow")
	}
	result, err := handle.GetResult()
	if err != nil {
		return "", errors.Wrap(err, "get greet result")
	}
	return result, nil
}

func Greet(ctx dbos.DBOSContext, name string) (string, error) {
	greeting, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		return "Hello, " + name + "!", nil
	}, dbos.WithStepName("compose"))
	if err != nil {
		return "", errors.Wrap(err, "compose greeting")
	}
	return greeting, nil
}

func dsn(path string) string {
	if path == ":memory:" {
		return path
	}
	v := url.Values{}
	v.Set("_pragma", "journal_mode(WAL)")
	v.Add("_pragma", "busy_timeout(5000)")
	v.Add("_txlock", "immediate")
	return "file:" + path + "?" + v.Encode()
}
