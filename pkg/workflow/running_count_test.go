package workflow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRunningCount(t *testing.T) {
	r := require.New(t)
	w, err := New(filepath.Join(t.TempDir(), "workflows.db"))
	r.NoError(err)
	r.NoError(w.Launch())
	defer w.Close()

	for _, status := range []string{"CANCELLED", "DELAYED", "ENQUEUED", "ERROR", "PENDING", "SUCCESS"} {
		_, err := w.conn.Exec("INSERT INTO workflow_status (workflow_uuid, status, name, created_at, updated_at) VALUES (?, ?, ?, 1, 1)", status, status, "test")
		r.NoError(err)
	}
	count, err := w.RunningCount()
	r.NoError(err)
	r.Equal(1, count)

	_, err = w.conn.Exec("UPDATE workflow_status SET status = 'SUCCESS' WHERE workflow_uuid = 'PENDING'")
	r.NoError(err)
	count, err = w.RunningCount()
	r.NoError(err)
	r.Zero(count)
}
