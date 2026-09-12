package inbox

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/robfig/cron/v3"
)

func (s *Server) ConfigureRunHistory(workflows *workflow.Workflows) {
	s.workflows = workflows
}

func (s *Server) ConfigureWorkflows(workflows *workflow.Workflows, store db.ThreadStore) error {
	threads, err := store.ListThreads(db.ThreadKindChat)
	if err != nil {
		return err
	}
	s.scheduleIDs = make(map[int64]string)
	s.workflows = workflows
	for _, def := range workflows.ScheduleDefinitions() {
		agent := "workflow:schedule:" + def.Name
		var id int64
		for _, thread := range threads {
			if s.chat.AgentName(thread) == agent {
				id = thread.ID
				break
			}
		}
		if id == 0 {
			metadata, err := json.Marshal(map[string]string{"agent": agent})
			if err != nil {
				return err
			}
			thread, err := store.AddThread(db.ThreadKindChat, def.Title, string(metadata))
			if err != nil {
				return err
			}
			id = thread.ID
		}
		s.scheduleIDs[id] = def.Name
	}
	return nil
}

func (s *Server) scheduleID(platform string) int64 {
	for id, source := range s.scheduleIDs {
		if source == platform {
			return id
		}
	}
	return 0
}

func (s *Server) handleScheduleAction(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if s.workflows == nil || s.scheduleIDs[id] == "" {
		http.NotFound(w, r)
		return
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Cross-site submission rejected", 403)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != "null" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host {
			http.Error(w, "Cross-site submission rejected", 403)
			return
		}
	}
	action := r.PathValue("action")
	if action != "run" && action != "pause" && action != "resume" {
		http.NotFound(w, r)
		return
	}
	if err := s.workflows.ScheduleAction(s.scheduleIDs[id], action); err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, "/inbox/workflows/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) scheduleDetail(id int64) (ui.InboxScheduleDetail, error) {
	thread, err := s.chat.Thread(id)
	if err != nil {
		return ui.InboxScheduleDetail{}, err
	}
	schedule, runs, err := s.workflows.ScheduleStatus(s.scheduleIDs[id])
	if err != nil {
		return ui.InboxScheduleDetail{}, err
	}
	if schedule == nil {
		return ui.InboxScheduleDetail{}, errors.New("workflow schedule missing")
	}
	detail := ui.InboxScheduleDetail{Archived: thread.State == db.ThreadStateArchived, ID: id,
		NextRun: "Paused", Starred: thread.Starred, Status: "Paused", Title: thread.Title}
	detail.ScheduleLabel, err = describeSchedule(schedule.Schedule, schedule.CronTimezone)
	if err != nil {
		return detail, err
	}
	detail.TotalRuns, err = s.workflows.ScheduleRunCount(s.scheduleIDs[id])
	if err != nil {
		return detail, err
	}
	detail.Created = thread.CreatedAt.Format("Jan 2")
	detail.Updated = thread.UpdatedAt.Format("Jan 2")
	detail.Cron = schedule.Schedule
	def, ok := s.workflows.ScheduleDefinition(s.scheduleIDs[id])
	if !ok {
		return detail, errors.New("workflow definition missing")
	}
	detail.Steps = def.Steps
	detail.Timezone = schedule.CronTimezone
	detail.LastRun = "Never"
	if len(runs) > 0 {
		detail.LastRun = runs[0].CreatedAt.UTC().Format("Jan 2, 2006 at 15:04 UTC")
	}
	successful, recent, err := s.workflows.ScheduleRunStats(s.scheduleIDs[id])
	if err != nil {
		return detail, err
	}
	detail.Last24h = recent
	detail.SuccessRate = "—"
	if detail.TotalRuns > 0 {
		detail.SuccessRate = fmt.Sprintf("%.0f%%", 100*float64(successful)/float64(detail.TotalRuns))
	}
	detail.Triggers = "Timer (paused)"
	detail.Description = def.Description
	if schedule.Status == "ACTIVE" {
		detail.Status = "Active"
		detail.Triggers = "Timer"
		parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		zone := schedule.CronTimezone
		if zone == "" {
			zone = "UTC"
		}
		location, err := time.LoadLocation(zone)
		if err != nil {
			return detail, err
		}
		parsed, err := parser.Parse("CRON_TZ=" + zone + " " + schedule.Schedule)
		if err != nil {
			return detail, err
		}
		detail.NextRun = parsed.Next(time.Now().In(location)).Format("Jan 2, 2006 at 15:04 MST")
	}
	fields := s.workflows.RunSchema(s.scheduleIDs[id])
	for _, field := range fields {
		detail.RunColumns = append(detail.RunColumns, field.Label)
	}
	for _, run := range runs {
		result := "Waiting to start."
		switch run.Status {
		case "PENDING":
			result = "Running workflow."
		case "SUCCESS":
			result = "Workflow completed."
		case "ERROR":
			result = "Workflow failed."
			if run.Error != nil {
				result = run.Error.Error()
			}
		case "CANCELLED":
			result = "Run cancelled."
		}
		summary := workflow.SummarizeRun(run, fields)
		detail.Runs = append(detail.Runs, ui.InboxScheduleRun{Cells: summary.Cells, At: run.CreatedAt.UTC().Format("Jan 2, 2006 at 15:04 UTC"), ID: run.ID, Result: result, Status: summary.Status})
	}
	return detail, nil
}
