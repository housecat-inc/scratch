package workflow_test

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite"
)

func TestSingleDBSharesSchemas(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "scratch.db")

	store, err := db.New(path)
	r.NoError(err)
	defer store.Close()

	w, err := workflow.New(path)
	r.NoError(err)
	defer w.Close()
	r.NoError(w.Launch())

	_, err = w.Greet("Ada")
	r.NoError(err)

	conn, err := sql.Open("sqlite", "file:"+path)
	r.NoError(err)
	defer conn.Close()

	tables := []struct {
		name  string
		table string
	}{
		{name: "app", table: "comments"},
		{name: "workflow", table: "workflow_status"},
	}
	for _, tt := range tables {
		t.Run(tt.name, func(t *testing.T) {
			var name string
			err := conn.QueryRow(
				`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`,
				tt.table,
			).Scan(&name)
			assert.New(t).NoError(err)
		})
	}

	var appTables, workflowRows int
	r.NoError(conn.QueryRow(`SELECT
		(SELECT count(*) FROM sqlite_master WHERE type = 'table' AND name = 'comments'),
		(SELECT count(*) FROM workflow_status)`).Scan(&appTables, &workflowRows))
	a.Equal(1, appTables)
	a.Positive(workflowRows)
}
