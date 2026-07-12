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
