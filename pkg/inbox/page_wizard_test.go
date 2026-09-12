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

func TestPageWizardChatHandoff(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	chats := chat.NewService(store, chat.EchoAgent{}, nil)
	t.Cleanup(chats.Close)
	handler := NewServer(todo.NewService(store), chats, nil).Handler()
	page := httptest.NewRecorder()
	handler.ServeHTTP(page, httptest.NewRequest("GET", "/pages/new", nil))
	r.Equal(http.StatusOK, page.Code)
	a.Contains(page.Body.String(), "Build your page")
	values := url.Values{"title": {"Team dashboard"}, "description": {"Track our projects"}, "details": {"Group by status"}}
	req := httptest.NewRequest("POST", "/pages/new", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	r.Equal(http.StatusSeeOther, response.Code)
	a.True(strings.HasPrefix(response.Header().Get("Location"), "/inbox/chats/"))
	reader := httptest.NewRecorder()
	handler.ServeHTTP(reader, httptest.NewRequest("GET", response.Header().Get("Location"), nil))
	r.Equal(http.StatusOK, reader.Code)
	a.Contains(reader.Body.String(), "Track our projects")
	a.Contains(reader.Body.String(), "Group by status")
	a.Contains(reader.Body.String(), "Page name: Team dashboard")
}

func TestPageWizardValidation(t *testing.T) {
	for _, tt := range []struct {
		name      string
		values    url.Values
		wantError bool
	}{
		{name: "minimal", values: url.Values{"title": {"Reading list"}, "description": {"Track books"}}},
		{name: "missing title", values: url.Values{"description": {"Track books"}}, wantError: true},
		{name: "blank description", values: url.Values{"title": {"Reading list"}, "description": {"  "}}, wantError: true},
		{name: "long title", values: url.Values{"title": {strings.Repeat("x", 201)}, "description": {"Track books"}}, wantError: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			prompt, err := pageWizardPrompt(tt.values)
			if tt.wantError {
				a.Error(err)
				return
			}
			a.NoError(err)
			a.Contains(prompt, "Reading list")
			a.Contains(prompt, "Suggest a useful starting layout")
		})
	}
}

func TestPageWizardRejectsInvalidRequests(t *testing.T) {
	for _, tt := range []struct {
		body   string
		name   string
		origin string
		site   string
		status int
	}{
		{name: "missing fields", status: http.StatusBadRequest},
		{name: "cross-site", site: "cross-site", status: http.StatusForbidden},
		{name: "foreign origin", origin: "https://other.example", status: http.StatusForbidden},
		{name: "oversized", body: strings.Repeat("x", 129<<10), status: http.StatusBadRequest},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", "/pages/new", strings.NewReader(tt.body))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Sec-Fetch-Site", tt.site)
			response := httptest.NewRecorder()
			(&Server{}).handlePageWizardSubmit(response, req)
			assert.Equal(t, tt.status, response.Code)
		})
	}
}
