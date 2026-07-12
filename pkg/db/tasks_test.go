package db

import (
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/ts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskCRUD(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	restore := func() func() {
		_, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
		return restore
	}()
	defer restore()

	d := newTestDB(t)

	first, err := d.AddTask("write tests")
	r.NoError(err)
	a.NotEmpty(first.ID)
	a.Equal("write tests", first.Title)
	a.False(first.Completed)
	a.Equal("2026-07-11T12:00:00Z", first.CreatedAt.Format(time.RFC3339))

	_, err = d.AddTask("ship feature")
	r.NoError(err)

	list, err := d.ListTasks()
	r.NoError(err)
	r.Len(list, 2)
	a.Equal("write tests", list[0].Title)

	toggled, err := d.SetTaskCompleted(first.ID, true)
	r.NoError(err)
	a.True(toggled.Completed)

	edited, err := d.UpdateTaskTitle(first.ID, "write more tests")
	r.NoError(err)
	a.Equal("write more tests", edited.Title)

	n, err := d.ClearCompletedTasks()
	r.NoError(err)
	a.Equal(1, n)

	list, err = d.ListTasks()
	r.NoError(err)
	r.Len(list, 1)
	a.Equal("ship feature", list[0].Title)
}

func TestTaskErrors(t *testing.T) {
	tests := []struct {
		act  func(d *DB) error
		name string
	}{
		{act: func(d *DB) error { return d.DeleteTask(404) }, name: "delete missing"},
		{act: func(d *DB) error { _, err := d.SetTaskCompleted(404, true); return err }, name: "toggle missing"},
		{act: func(d *DB) error { _, err := d.UpdateTaskTitle(404, "x"); return err }, name: "update missing"},
		{act: func(d *DB) error { _, err := d.GetTask(404); return err }, name: "get missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			d := newTestDB(t)
			a.True(IsTaskNotFound(tc.act(d)))
		})
	}
}
