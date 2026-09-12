package workflow

import (
	"sort"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/robfig/cron/v3"
)

type StepDefinition struct {
	Description string
	Name        string
}

type ScheduleDefinition struct {
	Description string
	Name        string
	Run         func(dbos.DBOSContext, dbos.ScheduledWorkflowInput) (any, error)
	Schedule    string
	Schema      []SummaryField
	Steps       []StepDefinition
	Timezone    string
	Title       string
}

func (w *Workflows) RegisterSchedule(def ScheduleDefinition) error {
	if def.Name == "" || def.Title == "" || def.Run == nil {
		return errors.New("scheduled workflows require a name, title, and function")
	}
	if _, ok := w.schedules[def.Name]; ok {
		return errors.New("workflow schedule already registered")
	}
	if def.Timezone == "" {
		def.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(def.Timezone); err != nil {
		return errors.Wrap(err, "schedule timezone")
	}
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(def.Schedule); err != nil {
		return errors.Wrap(err, "schedule expression")
	}
	if len(def.Schema) > 0 {
		if err := w.SetRunSchema(def.Name, def.Schema); err != nil {
			return err
		}
	}
	if w.schedules == nil {
		w.schedules = make(map[string]ScheduleDefinition)
	}
	dbos.RegisterWorkflow(w.ctx, def.Run, dbos.WithWorkflowName(def.Name))
	w.schedules[def.Name] = def
	return nil
}

func (w *Workflows) ScheduleDefinitions() []ScheduleDefinition {
	definitions := make([]ScheduleDefinition, 0, len(w.schedules))
	for _, def := range w.schedules {
		definitions = append(definitions, def)
	}
	sort.Slice(definitions, func(i, j int) bool { return definitions[i].Name < definitions[j].Name })
	return definitions
}

func (w *Workflows) ScheduleDefinition(name string) (ScheduleDefinition, bool) {
	def, ok := w.schedules[name]
	return def, ok
}

func (w *Workflows) EnsureSchedules() error {
	schedules, err := dbos.ListSchedules(w.ctx)
	if err != nil {
		return errors.Wrap(err, "list schedules")
	}
	existing := make(map[string]bool)
	for _, schedule := range schedules {
		existing[schedule.ScheduleName] = true
	}
	for _, def := range w.ScheduleDefinitions() {
		if existing[def.Name] {
			continue
		}
		if err := dbos.CreateSchedule(w.ctx, def.Run, dbos.CreateScheduleRequest{
			Schedule: def.Schedule, ScheduleName: def.Name,
		}, dbos.WithAutomaticBackfill(false), dbos.WithCronTimezone(def.Timezone)); err != nil {
			return errors.Wrap(err, "create schedule")
		}
		if err := dbos.PauseSchedule(w.ctx, def.Name); err != nil {
			return errors.Wrap(err, "pause new schedule")
		}
	}
	return nil
}

func (w *Workflows) ScheduleStatus(name string) (*dbos.WorkflowSchedule, []dbos.WorkflowStatus, error) {
	schedule, err := dbos.GetSchedule(w.ctx, name)
	if err != nil {
		return nil, nil, err
	}
	runs, err := dbos.ListWorkflows(w.ctx, dbos.WithName(name), dbos.WithLimit(20), dbos.WithSortDesc(), dbos.WithLoadOutput(true))
	for i := range runs {
		runs[i].Output = summaryValue(runs[i].Output)
	}
	return schedule, runs, err
}

func (w *Workflows) ScheduleAction(name, action string) error {
	if _, ok := w.schedules[name]; !ok {
		return errors.New("unknown workflow schedule")
	}
	switch action {
	case "pause":
		return dbos.PauseSchedule(w.ctx, name)
	case "resume":
		return dbos.ResumeSchedule(w.ctx, name)
	case "run":
		_, err := dbos.TriggerSchedule(w.ctx, name)
		return err
	default:
		return errors.New("unknown workflow action")
	}
}

func (w *Workflows) ScheduleRunCount(name string) (int64, error) {
	var count int64
	err := w.conn.QueryRowContext(w.ctx, "SELECT COUNT(*) FROM workflow_status WHERE name = ?", name).Scan(&count)
	return count, errors.Wrap(err, "count workflow runs")
}

func (w *Workflows) ScheduleRunStats(name string) (successful, recent int64, err error) {
	err = w.conn.QueryRowContext(w.ctx, "SELECT COALESCE(SUM(CASE WHEN status = 'SUCCESS' THEN 1 ELSE 0 END), 0), COALESCE(SUM(CASE WHEN created_at >= ? THEN 1 ELSE 0 END), 0) FROM workflow_status WHERE name = ?", time.Now().Add(-24*time.Hour).UnixMilli(), name).Scan(&successful, &recent)
	return successful, recent, errors.Wrap(err, "load workflow run stats")
}
