package inbox

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/ui"
)

func (s *Server) handlePageWizard(w http.ResponseWriter, r *http.Request) {
	props, err := s.props("pages", "all", ui.InboxSelection{})
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.PageWizardPage(props))
}

func pageWizardPrompt(values url.Values) (string, error) {
	title := strings.TrimSpace(values.Get("title"))
	description := strings.TrimSpace(values.Get("description"))
	details := strings.TrimSpace(values.Get("details"))
	if title == "" || description == "" {
		return "", errors.New("give your page a name and describe what it should do")
	}
	if len(title) > 200 || len(description) > 12000 || len(details) > 12000 {
		return "", errors.New("page details are too long")
	}
	if details == "" {
		details = "Suggest a useful starting layout based on the page description."
	}
	return fmt.Sprintf("Build a new Scratch page using this setup brief.\n\nPage name: %s\n\nWhat it should do:\n%s\n\nContent, data, and actions:\n%s\n\nUse the existing workspace and match Scratch's page patterns. Implement the page and register it in the Pages library with a working route so it can be opened and pinned. Clarify any missing data connections as needed. This brief is a request to build the page, not confirmation that it already exists.", title, description, details), nil
}

func (s *Server) handlePageWizardSubmit(w http.ResponseWriter, r *http.Request) {
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
	r.Body = http.MaxBytesReader(w, r.Body, 128<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Could not read page details", http.StatusBadRequest)
		return
	}
	prompt, err := pageWizardPrompt(r.PostForm)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	r.PostForm = url.Values{"mode": {"chat"}, "prompt": {prompt}, "provider_model": {r.PostForm.Get("provider_model")}}
	r.Form = r.PostForm
	s.handleCompose(w, r)
}
