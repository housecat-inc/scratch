package flow

import (
	"context"
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	heartbeatCron = "0 * * * * *"
	heartbeatName = "heartbeat"
)

var ErrScheduleNotFound = errors.New("schedule not found")

type ScheduleView struct {
	Cron      string
	LastFired string
	Name      string
	Paused    bool
	Status    string
}

type ScheduleRunView struct {
	CreatedAt string
	ID        string
	Steps     []StepView
	Status    string
}

func (e *Engine) heartbeat(ctx dbos.DBOSContext, in dbos.ScheduledWorkflowInput) (any, error) {
	return dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		return "beat at " + in.ScheduledTime.Format(time.RFC3339), nil
	}, dbos.WithStepName("beat"))
}

func (e *Engine) EnsureSchedules() error {
	existing, err := dbos.GetSchedule(e.ctx, heartbeatName)
	if err != nil {
		return errors.Wrap(err, "get schedule")
	}
	if existing != nil {
		return nil
	}
	err = dbos.CreateSchedule(e.ctx, e.heartbeat, dbos.CreateScheduleRequest{
		Schedule:     heartbeatCron,
		ScheduleName: heartbeatName,
	})
	return errors.Wrap(err, "create schedule")
}

func (e *Engine) Schedules() ([]ScheduleView, error) {
	schedules, err := dbos.ListSchedules(e.ctx)
	if err != nil {
		return nil, errors.Wrap(err, "list schedules")
	}
	views := make([]ScheduleView, 0, len(schedules))
	for _, s := range schedules {
		view := ScheduleView{
			Cron:   s.Schedule,
			Name:   s.ScheduleName,
			Paused: s.Status == dbos.ScheduleStatusPaused,
			Status: string(s.Status),
		}
		if s.LastFiredAt != nil {
			view.LastFired = s.LastFiredAt.Format("15:04:05")
		}
		views = append(views, view)
	}
	sort.SliceStable(views, func(i, j int) bool { return views[i].Name < views[j].Name })
	return views, nil
}

func (e *Engine) PauseSchedule(name string) error {
	return errors.Wrap(dbos.PauseSchedule(e.ctx, name), "pause schedule")
}

func (e *Engine) ResumeSchedule(name string) error {
	return errors.Wrap(dbos.ResumeSchedule(e.ctx, name), "resume schedule")
}

func (e *Engine) ScheduleRuns(name string) ([]ScheduleRunView, error) {
	schedule, err := dbos.GetSchedule(e.ctx, name)
	if err != nil {
		return nil, errors.Wrap(err, "get schedule")
	}
	if schedule == nil {
		return nil, ErrScheduleNotFound
	}
	runs, err := dbos.ListWorkflows(
		e.ctx,
		dbos.WithLimit(25),
		dbos.WithSortDesc(),
		dbos.WithWorkflowIDPrefix("sched-"+name+"-"),
	)
	if err != nil {
		return nil, errors.Wrap(err, "list schedule runs")
	}
	views := make([]ScheduleRunView, 0, len(runs))
	for _, run := range runs {
		view, err := e.Run(run.ID)
		if err != nil {
			return nil, err
		}
		views = append(views, ScheduleRunView{
			CreatedAt: run.CreatedAt.Format("15:04:05"),
			ID:        run.ID,
			Steps:     view.Steps,
			Status:    view.Status,
		})
	}
	return views, nil
}

func (e *Engine) TriggerSchedule(name string) (string, error) {
	handle, err := dbos.TriggerSchedule(e.ctx, name)
	if err != nil {
		return "", errors.Wrap(err, "trigger schedule")
	}
	return handle.GetWorkflowID(), nil
}
