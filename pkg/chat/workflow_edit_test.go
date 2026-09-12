package chat

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflowEditPopout(t *testing.T) {
	for _, field := range []string{"schedule", "triggers", "invalid"} {
		t.Run(field, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			store, err := db.New(":memory:")
			r.NoError(err)
			defer store.Close()
			svc := NewService(store, EchoAgent{}, nil)
			defer svc.Close()
			workflow, err := store.AddThread(db.ThreadKindChat, "Example greeting", `{"agent":"workflow:schedule:example-greeting"}`)
			r.NoError(err)
			query := url.Values{"workflow_id": {strconv.FormatInt(workflow.ID, 10)}, "workflow_field": {field}, "workflow_value": {"Daily at 06:00 UTC"}}
			response := httptest.NewRecorder()
			NewServer(svc, nil).Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/chat/popout/new?"+query.Encode(), nil))
			if field == "invalid" {
				a.Equal(http.StatusBadRequest, response.Code)
				threads, err := svc.Threads()
				r.NoError(err)
				a.Len(threads, 1)
				return
			}
			r.Equal(http.StatusOK, response.Code)
			a.Contains(response.Body.String(), `id="floating-chat"`)
			a.Contains(response.Body.String(), "What would you like to change")
			a.Contains(response.Body.String(), "Example greeting")
			a.Contains(response.Body.String(), "Daily at 06:00 UTC")
			view, err := svc.View(2)
			r.NoError(err)
			r.Len(view.Messages, 1)
			a.Equal(db.MessageRoleAssistant, view.Messages[0].Role)
			a.Equal("echo", svc.AgentName(view.Thread))
			_, err = svc.Send(2, "Change it to noon")
			r.NoError(err)
			view = waitComplete(t, svc, 2)
			a.Contains(view.Messages[len(view.Messages)-1].Body, "Example greeting")
			a.Contains(view.Messages[len(view.Messages)-1].Body, "Change it to noon")
			a.Contains(mergeThreadAnchor(view.Thread.Anchor, `{"agent":"echo"}`), "workflow_edit")
		})
	}
}
