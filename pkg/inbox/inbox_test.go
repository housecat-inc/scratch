package inbox

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestResolveComposeMode(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		prompt     string
		view       string
		wantMode   string
		wantPrompt string
	}{
		{name: "explicit chat", mode: "chat", prompt: "hello", view: "tasks", wantMode: "chat", wantPrompt: "hello"},
		{name: "prefix task", mode: "auto", prompt: "task: follow up", view: "inbox", wantMode: "task", wantPrompt: "follow up"},
		{name: "prefix workflow", mode: "auto", prompt: "workflow: add contact", view: "inbox", wantMode: "workflow", wantPrompt: "add contact"},
		{name: "tasks view", mode: "auto", prompt: "follow up", view: "tasks", wantMode: "task", wantPrompt: "follow up"},
		{name: "workflows view", mode: "auto", prompt: "add contact", view: "workflows", wantMode: "workflow", wantPrompt: "add contact"},
		{name: "inbox defaults chat", mode: "auto", prompt: "hello", view: "inbox", wantMode: "chat", wantPrompt: "hello"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			mode, prompt := resolveComposeMode(tt.mode, tt.prompt, tt.view)
			a.Equal(tt.wantMode, mode)
			a.Equal(tt.wantPrompt, prompt)
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
		{agent: "contact", name: "workflow agent"},
		{agent: "missing", name: "unknown agent"},
		{agent: "", name: "default"},
	}
	agents := []string{"claude", "codex", "contact", "echo"}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, chatAgent(tt.agent, agents))
		})
	}
}

func TestChatAgentOptions(t *testing.T) {
	a := assert.New(t)

	a.Equal([]string{"claude", "codex", "echo"}, chatAgentOptions([]string{"claude", "codex", "contact", "echo"}))
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

func TestWorkflowAgent(t *testing.T) {
	tests := []struct {
		name string
		typ  string
		want string
	}{
		{name: "default", want: "contact"},
		{name: "contact", typ: "contact", want: "contact"},
		{name: "unknown", typ: "other", want: "contact"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, workflowAgent(tt.typ))
		})
	}
}
