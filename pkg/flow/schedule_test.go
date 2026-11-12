package flow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestSchedule(t *testing.T) {
	t.Run("heartbeat schedule", func(t *testing.T) {
		e := newEngine(t)

		t.Run("registers once when ensured twice", func(t *testing.T) {
			r := require.New(t)
			r.NoError(e.EnsureSchedules())
			r.NoError(e.EnsureSchedules())

			schedules, err := e.Schedules()
			r.NoError(err)
			r.Len(schedules, 1)
			r.Equal(heartbeatName, schedules[0].Name)
			r.Equal("ACTIVE", schedules[0].Status)
			r.False(schedules[0].Paused)
		})

		t.Run("pauses and resumes", func(t *testing.T) {
			r := require.New(t)
			r.NoError(e.PauseSchedule(heartbeatName))
			schedules, err := e.Schedules()
			r.NoError(err)
			r.True(schedules[0].Paused)
			r.Equal("PAUSED", schedules[0].Status)

			r.NoError(e.ResumeSchedule(heartbeatName))
			schedules, err = e.Schedules()
			r.NoError(err)
			r.False(schedules[0].Paused)
		})

		t.Run("records a triggered run in its history", func(t *testing.T) {
			r := require.New(t)
			runID, err := e.TriggerSchedule(heartbeatName)
			r.NoError(err)
			r.NotEmpty(runID)
			e.Await(runID, 5*time.Second)

			run, err := e.Run(runID)
			r.NoError(err)
			r.True(run.Done())

			runs, err := e.ScheduleRuns(heartbeatName)
			r.NoError(err)
			r.NotEmpty(runs)
			r.Equal(runID, runs[0].ID)
			r.Equal("SUCCESS", runs[0].Status)
			r.NotEmpty(runs[0].Steps)
		})
	})
}
