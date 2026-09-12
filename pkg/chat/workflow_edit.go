package chat

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"
)

func (s *Server) workflowEditQuestion(r *http.Request) (string, string, error) {
	rawID := r.URL.Query().Get("workflow_id")
	if rawID == "" {
		return "", "", nil
	}
	id, err := strconv.ParseInt(rawID, 10, 64)
	if err != nil {
		return "", "", errors.New("invalid workflow")
	}
	thread, err := s.svc.Thread(id)
	if err != nil {
		return "", "", err
	}
	if !strings.HasPrefix(s.svc.AgentName(thread), "workflow:") {
		return "", "", errors.New("thread is not a workflow")
	}
	field := r.URL.Query().Get("workflow_field")
	switch field {
	case "schedule":
		field = "cron schedule"
	case "triggers":
	default:
		return "", "", errors.New("invalid workflow setting")
	}
	title := "Edit " + field + " · " + thread.Title
	question := fmt.Sprintf("What would you like to change about the %s for %s?\n\nCurrent setting: %s\n\nWorkflow: /inbox/workflows/%d", field, thread.Title, r.URL.Query().Get("workflow_value"), id)
	return title, question, nil
}
