package inbox

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/stretchr/testify/assert"
)

func TestResolveComposeMode(t *testing.T) {
	tests := []struct {
		mode string
		want string
	}{
		{mode: "", want: "chat"},
		{mode: "chat", want: "chat"},
		{mode: "task", want: "task"},
		{mode: "workflow", want: "workflow"},
		{mode: "other", want: "chat"},
	}
	for _, tt := range tests {
		t.Run(tt.mode, func(t *testing.T) {
			assert.New(t).Equal(tt.want, resolveComposeMode(tt.mode))
		})
	}
}

func TestChatAgent(t *testing.T) {
	tests := []struct {
		agent string
		name  string
		want  string
	}{
		{agent: "codex", name: "known chat agent", want: "codex"},
		{agent: "missing", name: "unknown agent"},
		{agent: "", name: "default"},
	}
	agents := []string{"claude", "codex", "echo"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, chatAgent(tt.agent, agents))
		})
	}
}

func TestChatAgentOptions(t *testing.T) {
	a := assert.New(t)

	a.Equal([]string{"claude", "codex", "echo"}, chatAgentOptions([]string{"claude", "codex", "echo"}))
}

func TestChatModel(t *testing.T) {
	tests := []struct {
		agent string
		model string
		name  string
		want  string
	}{
		{agent: "codex", model: "gpt-5.5", name: "codex model", want: "gpt-5.5"},
		{agent: "claude", model: "opus", name: "claude model", want: "opus"},
		{agent: "claude", model: "gpt-5.5", name: "wrong provider"},
		{agent: "codex", model: "default", name: "default"},
		{agent: "echo", model: "opus", name: "unsupported"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, chatModel(tt.agent, tt.model))
		})
	}
}

func TestArchiveFilter(t *testing.T) {
	tests := []struct {
		filter string
		name   string
		view   string
		want   string
	}{
		{filter: "active", name: "active group filter", view: "chats", want: "active"},
		{filter: "archived", name: "archived group filter", view: "tasks", want: "archived"},
		{filter: "other", name: "unknown group filter defaults all", view: "workflows", want: "all"},
		{filter: "archived", name: "inbox ignores filter", view: "inbox", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, archiveFilter(tt.view, tt.filter))
		})
	}
}

func TestScheduleLastFired(t *testing.T) {
	a := assert.New(t)
	now := time.Now()

	a.Equal("", scheduleLastFired(time.Time{}))
	a.Equal("last at "+now.Local().Format("3:04 PM"), scheduleLastFired(now))
}

func TestLegacyTaskPagesAreNotMounted(t *testing.T) {
	a := assert.New(t)
	srv := NewServer(nil, nil, nil, nil).Handler()

	for _, path := range []string{"/tasks", "/tasks/1"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		a.Equal(http.StatusNotFound, rec.Code, path)
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
			item := ui.InboxItem{Archived: tt.archived, Done: tt.done}
			a.Equal(tt.want, includeTaskFilter(tt.filter, item))
		})
	}
}

func TestWorkflowAgent(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want string
	}{
		{name: "default", want: "greet"},
		{name: "greet", typ: "greet", want: "greet"},
		{name: "unknown", typ: "other", want: "greet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, workflowAgent(tt.typ))
		})
	}
}
