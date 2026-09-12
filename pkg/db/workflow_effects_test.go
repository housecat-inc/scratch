package db

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkflowEffectsAreAtomicAndIdempotent(t *testing.T) {
	r := require.New(t)
	d, err := New(filepath.Join(t.TempDir(), "scratch.db"))
	r.NoError(err)
	defer d.Close()
	first, err := d.AddWorkflowTask("run/task", "Follow up")
	r.NoError(err)
	second, err := d.AddWorkflowTask("run/task", "Follow up")
	r.NoError(err)
	r.Equal(first, second)
	tasks, err := d.ListTasks()
	r.NoError(err)
	r.Len(tasks, 1)
	thread, err := d.AddThread(ThreadKindChat, "fixture", "{}")
	r.NoError(err)
	message, err := d.AddMessage(NewMessage{ThreadID: thread.ID, Role: "assistant", Author: "workflow:contact"})
	r.NoError(err)
	for range 2 {
		r.NoError(d.AddWorkflowMessageEvent("run/delta", message.ID, "delta", `{"text":"saved"}`, "saved"))
	}
	var body string
	r.NoError(d.conn.QueryRow("SELECT body FROM messages WHERE id=?", message.ID).Scan(&body))
	r.Equal("saved", body)
	var count int
	r.NoError(d.conn.QueryRow("SELECT COUNT(*) FROM message_events WHERE message_id=?", message.ID).Scan(&count))
	r.Equal(1, count)
	r.Error(d.AddWorkflowMessageEvent("run/invalid", 999999, "delta", "{}", "bad"))
	r.NoError(d.conn.QueryRow("SELECT COUNT(*) FROM workflow_effects WHERE effect_key='run/invalid'").Scan(&count))
	r.Zero(count)
}
