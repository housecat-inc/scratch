package inbox

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/stretchr/testify/assert"
)

func TestNewServer(t *testing.T) {
	t.Run("does not mount the legacy task pages", func(t *testing.T) {
		a := assert.New(t)
		srv := NewServer(nil, nil, nil, nil).Handler()

		for _, path := range []string{"/tasks", "/tasks/1"} {
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			a.Equal(http.StatusNotFound, rec.Code, path)
		}
	})
}

func TestArchiveFilter(t *testing.T) {
	tests := []struct {
		filter string
		name   string
		view   string
		want   string
	}{
		{filter: "", name: "defaults chats to active", view: "chats", want: "active"},
		{filter: "all", name: "keeps the all filter for chats", view: "chats", want: "all"},
		{filter: "archived", name: "keeps the archived filter for tasks", view: "tasks", want: "archived"},
		{filter: "other", name: "defaults an unknown filter to all", view: "workflows", want: "all"},
		{filter: "archived", name: "ignores the filter in the inbox view", view: "inbox", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, archiveFilter(tt.view, tt.filter))
		})
	}
}

func TestIncludeArchiveFilter(t *testing.T) {
	tests := []struct {
		archived bool
		filter   string
		name     string
		want     bool
	}{
		{filter: "all", name: "all includes active", want: true},
		{archived: true, filter: "all", name: "all includes archived", want: true},
		{filter: "active", name: "active includes active", want: true},
		{archived: true, filter: "active", name: "active excludes archived"},
		{filter: "archived", name: "archived excludes active"},
		{archived: true, filter: "archived", name: "archived includes archived", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, includeArchiveFilter(tt.filter, ui.InboxItem{Archived: tt.archived}))
		})
	}
}

func TestIncludeTaskFilter(t *testing.T) {
	tests := []struct {
		archived bool
		done     bool
		filter   string
		name     string
		want     bool
	}{
		{filter: "all", name: "all includes active", want: true},
		{done: true, filter: "all", name: "all includes done", want: true},
		{filter: "active", name: "active includes open", want: true},
		{done: true, filter: "active", name: "active excludes done"},
		{archived: true, filter: "active", name: "active excludes archived"},
		{filter: "archived", name: "archived excludes open"},
		{archived: true, filter: "archived", name: "archived includes archived", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, includeTaskFilter(tt.filter, ui.InboxItem{Archived: tt.archived, Done: tt.done}))
		})
	}
}

func TestResolveComposeMode(t *testing.T) {
	tests := []struct {
		mode string
		name string
		want string
	}{
		{mode: "", name: "is empty without a mode", want: ""},
		{mode: "chat", name: "rejects the retired chat mode", want: ""},
		{mode: "task", name: "keeps task", want: "task"},
		{mode: "workflow", name: "keeps workflow", want: "workflow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, resolveComposeMode(tt.mode))
		})
	}
}

func TestScheduleLastFired(t *testing.T) {
	t.Run("formats the last fire time", func(t *testing.T) {
		a := assert.New(t)
		now := time.Now()
		fired := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, time.Local)
		a.Equal("last at "+fired.Format("3:04 PM"), scheduleLastFired(fired))
	})

	t.Run("is empty before the first fire", func(t *testing.T) {
		a := assert.New(t)
		a.Equal("", scheduleLastFired(time.Time{}))
	})
}

func TestWorkflowAgent(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want string
	}{
		{name: "defaults to greet", want: "greet"},
		{name: "keeps greet", typ: "greet", want: "greet"},
		{name: "falls back to greet for an unknown type", typ: "other", want: "greet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, workflowAgent(tt.typ))
		})
	}
}
