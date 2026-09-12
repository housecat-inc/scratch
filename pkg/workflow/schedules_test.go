package workflow

import (
	"path/filepath"
	"testing"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/stretchr/testify/require"
)

func TestSchedulesSurviveRestart(t *testing.T) {
	for _, action := range []string{"pause", "resume"} {
		t.Run(action, func(t *testing.T) {
			r := require.New(t)
			path := filepath.Join(t.TempDir(), "scratch.db")
			first, err := New(path)
			r.NoError(err)
			r.NoError(first.ConfigureExamples())
			r.NoError(first.Launch())
			r.NoError(first.EnsureSchedules())
			schedule, _, err := first.ScheduleStatus("example-greeting")
			r.NoError(err)
			r.Equal(dbos.ScheduleStatusPaused, schedule.Status)
			r.NoError(first.ScheduleAction("example-greeting", action))
			handle, err := dbos.TriggerSchedule(first.Ctx(), "example-greeting")
			r.NoError(err)
			_, err = handle.GetResult()
			r.NoError(err)
			before, _, err := first.ScheduleStatus("example-greeting")
			r.NoError(err)
			r.NoError(first.Close())
			second, err := New(path)
			r.NoError(err)
			def := ScheduleDefinition{Name: "example-greeting", Title: "Example greeting", Run: scheduledGreeting, Schedule: "0 30 10 * * *", Timezone: "Europe/London"}
			r.NoError(second.RegisterSchedule(def))
			r.NoError(second.Launch())
			t.Cleanup(func() { second.Close() })
			r.NoError(second.EnsureSchedules())
			after, runs, err := second.ScheduleStatus("example-greeting")
			r.NoError(err)
			r.Equal(before.Schedule, after.Schedule)
			r.Equal(before.CronTimezone, after.CronTimezone)
			r.Equal(before.Status, after.Status)
			r.Len(runs, 1)
			r.Equal(handle.GetWorkflowID(), runs[0].ID)
			schedules, err := dbos.ListSchedules(second.Ctx())
			r.NoError(err)
			r.Len(schedules, 1)
		})
	}
}

func TestRegisterScheduleValidation(t *testing.T) {
	for _, tc := range []struct {
		change func(*ScheduleDefinition)
		name   string
	}{
		{func(d *ScheduleDefinition) { d.Schedule = "invalid" }, "invalid cron"},
		{func(d *ScheduleDefinition) { d.Timezone = "invalid" }, "invalid timezone"},
		{func(d *ScheduleDefinition) { d.Name = "" }, "missing name"},
		{func(d *ScheduleDefinition) { d.Run = nil }, "missing function"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			w, err := New(filepath.Join(t.TempDir(), "scratch.db"))
			r.NoError(err)
			t.Cleanup(func() { w.Close() })
			def := ScheduleDefinition{Name: "example", Title: "Example", Run: scheduledGreeting, Schedule: "0 0 9 * * *"}
			tc.change(&def)
			r.Error(w.RegisterSchedule(def))
			r.Empty(w.ScheduleDefinitions())
		})
	}
}
