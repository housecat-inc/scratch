package inbox

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func TestScheduledWorkflowInbox(t *testing.T) {
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "scratch.db")
	store, err := db.New(path)
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	flows, err := workflow.New(path)
	r.NoError(err)
	r.NoError(flows.ConfigureExamples())
	r.NoError(flows.Launch())
	t.Cleanup(func() { flows.Close() })
	r.NoError(flows.EnsureSchedules())
	chats := chat.NewService(store, chat.EchoAgent{}, nil)
	t.Cleanup(chats.Close)
	server := NewServer(todo.NewService(store), chats, nil)
	r.NoError(server.ConfigureWorkflows(flows, store))
	r.NoError(server.ConfigureWorkflows(flows, store))
	threads, err := chats.Threads()
	r.NoError(err)
	r.Len(threads, 1)
	detailURL := "/inbox/workflows/" + strconv.FormatInt(server.scheduleID("example-greeting"), 10)
	handler := server.Handler()
	get := func(path string) string {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		r.Equal(http.StatusOK, response.Code)
		return response.Body.String()
	}
	r.Contains(get("/inbox/workflows"), "Example greeting")
	detail := get(detailURL)
	for _, expected := range []string{"Run now", "Resume schedule", "Daily at 09:00 (UTC)", "compose-greeting", "No runs yet", "Greeting", "Workflow statistics"} {
		r.Contains(detail, expected)
	}
	r.NotContains(detail, `class="chat-composer`)
	handle, err := dbos.TriggerSchedule(flows.Ctx(), "example-greeting")
	r.NoError(err)
	_, err = handle.GetResult()
	r.NoError(err)
	detail = get(detailURL)
	r.Contains(detail, "Hello from Scratch!")
	r.Contains(detail, "Completed successfully")
	r.Contains(detail, "<span>Total runs</span><strong>1</strong>")
	r.Contains(detail, "<span>Success rate</span><strong>100%</strong>")
	r.Contains(detail, "/workflows/audit?run="+url.QueryEscape(handle.GetWorkflowID()))
	for _, tc := range []struct{ action, status string }{{"resume", "ACTIVE"}, {"pause", "PAUSED"}} {
		t.Run(tc.action, func(t *testing.T) {
			r := require.New(t)
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, detailURL+"/schedule/"+tc.action, nil))
			r.Equal(http.StatusSeeOther, response.Code)
			r.Equal(detailURL, response.Header().Get("Location"))
			schedule, _, err := flows.ScheduleStatus("example-greeting")
			r.NoError(err)
			r.Equal(tc.status, string(schedule.Status))
		})
	}
	for _, action := range []string{"star", "archive"} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, detailURL+"/"+action, nil))
		r.Equal(http.StatusSeeOther, response.Code)
	}
	r.NotContains(get("/inbox/workflows?archived=active"), `class="gm-row-link" href="`+detailURL+`"`)
	r.Contains(get("/inbox/workflows?archived=archived"), "Example greeting")
	request := httptest.NewRequest(http.MethodPost, detailURL+"/schedule/resume", nil)
	request.Header.Set("Origin", "https://foreign.example")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	r.Equal(http.StatusForbidden, response.Code)
}
