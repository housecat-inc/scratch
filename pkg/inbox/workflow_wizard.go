package inbox

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/ui"
)

func (s *Server) handleWorkflowWizard(w http.ResponseWriter, r *http.Request) {
	props, err := s.props("workflows", "active", ui.InboxSelection{})
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	props.WorkflowWizard = true
	s.render(w, r, ui.InboxPage(props))
}

func workflowWizardPrompt(values url.Values) (string, error) {
	description := strings.TrimSpace(values.Get("description"))
	timing := strings.TrimSpace(values.Get("timing"))
	trigger := values.Get("trigger")
	zone := strings.TrimSpace(values.Get("timezone"))
	if description == "" || timing == "" {
		return "", errors.New("describe the workflow and when it should run")
	}
	switch trigger {
	case "external":
		trigger = "External trigger"
		zone = "Not applicable"
	case "schedule":
		trigger = "Time-based schedule"
		if zone == "" {
			return "", errors.New("a timezone is required for a schedule")
		}
		if _, err := time.LoadLocation(zone); err != nil {
			return "", errors.New("use a valid timezone such as America/Los_Angeles or UTC")
		}
	default:
		return "", errors.New("choose a schedule or external trigger")
	}
	return fmt.Sprintf("Build a Scratch workflow using this setup brief.\n\nWhat it should do:\n%s\n\nTrigger: %s\nWhen it should happen: %s\nTimezone: %s\n\nUse durable workflow execution and expose the workflow, its controls, and run history in Scratch. Use deterministic steps, LLM calls, or agent flows as appropriate. Clarify any missing connections or trigger details before enabling it. This brief is a request to build the workflow, not confirmation that it already exists.", description, trigger, timing, zone), nil
}

func (s *Server) handleWorkflowWizardSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		http.Error(w, "Cross-site submission rejected", http.StatusForbidden)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		parsed, err := url.Parse(origin)
		if err != nil || parsed.Host != r.Host {
			http.Error(w, "Cross-site submission rejected", http.StatusForbidden)
			return
		}
	}
	r.Body = http.MaxBytesReader(w, r.Body, 32<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read workflow details", http.StatusBadRequest)
		return
	}
	prompt, err := workflowWizardPrompt(r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.PostForm = url.Values{"mode": {"chat"}, "prompt": {prompt}, "provider_model": {r.PostForm.Get("provider_model")}}
	r.Form = r.PostForm
	s.handleCompose(w, r)
}
