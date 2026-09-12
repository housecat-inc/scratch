package inbox

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowWizardChatHandoff(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	chats := chat.NewService(store, chat.EchoAgent{}, nil)
	t.Cleanup(chats.Close)
	handler := NewServer(todo.NewService(store), chats, nil).Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/inbox/workflows/new", nil))
	r.Equal(http.StatusOK, page.Code)
	a.Contains(page.Body.String(), "Build your workflow")
	values := url.Values{"description": {"Prepare a weekly report"}, "timing": {"Monday at 9 AM"}, "trigger": {"schedule"}, "timezone": {"UTC"}}
	req := httptest.NewRequest("POST", "/inbox/workflows/new", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	r.Equal(http.StatusSeeOther, response.Code)
	a.True(strings.HasPrefix(response.Header().Get("Location"), "/inbox/chats/"))
	reader := httptest.NewRecorder()
	handler.ServeHTTP(reader, httptest.NewRequest("GET", response.Header().Get("Location"), nil))
	r.Equal(http.StatusOK, reader.Code)
	a.Contains(reader.Body.String(), "Prepare a weekly report")
	a.Contains(reader.Body.String(), "Monday at 9 AM")
	a.Contains(reader.Body.String(), "Timezone: UTC")
}

func TestWorkflowWizardPrompt(t *testing.T) {
	for _, tt := range []struct {
		name      string
		trigger   string
		zone      string
		wantError bool
	}{
		{name: "schedule", trigger: "schedule", zone: "America/Los_Angeles"},
		{name: "external without timezone", trigger: "external"},
		{name: "invalid timezone", trigger: "schedule", zone: "Mars/Olympus", wantError: true},
		{name: "missing timezone", trigger: "schedule", wantError: true},
		{name: "invalid trigger", trigger: "unknown", wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			values := url.Values{"description": {"Review feedback"}, "timing": {"When needed"}, "trigger": {tt.trigger}, "timezone": {tt.zone}}
			prompt, err := workflowWizardPrompt(values)
			if tt.wantError {
				a.Error(err)
				return
			}
			a.NoError(err)
			a.Contains(prompt, "Review feedback")
			a.Contains(prompt, "When needed")
			values.Set("description", "  ")
			_, err = workflowWizardPrompt(values)
			a.Error(err)
		})
	}
}
