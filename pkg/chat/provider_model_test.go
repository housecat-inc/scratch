package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseProviderModel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		agent string
		model string
		name  string
		value string
	}{
		{agent: "codex", model: "", name: "default", value: "codex:default"},
		{agent: "codex", model: "gpt-5.5", name: "model", value: "codex:gpt-5.5"},
		{agent: "echo", model: "", name: "agent only", value: "echo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)
			agent, model := ParseProviderModel(tt.value)
			a.Equal(tt.agent, agent)
			a.Equal(tt.model, model)
		})
	}
}

func TestProviderModelOptions(t *testing.T) {
	t.Parallel()

	a := assert.New(t)
	options := ProviderModelOptions([]string{"claude", "codex", "contact", "echo"})
	a.Equal([]ProviderModelOption{
		{Agent: "claude", Label: "Claude Default", Model: "", Value: "claude:default"},
		{Agent: "claude", Label: "Claude Haiku", Model: "haiku", Value: "claude:haiku"},
		{Agent: "claude", Label: "Claude Opus", Model: "opus", Value: "claude:opus"},
		{Agent: "claude", Label: "Claude Sonnet", Model: "sonnet", Value: "claude:sonnet"},
		{Agent: "codex", Label: "Codex Default", Model: "", Value: "codex:default"},
		{Agent: "codex", Label: "Codex GPT 5", Model: "gpt-5", Value: "codex:gpt-5"},
		{Agent: "codex", Label: "Codex GPT 5.1", Model: "gpt-5.1", Value: "codex:gpt-5.1"},
		{Agent: "codex", Label: "Codex GPT 5.5", Model: "gpt-5.5", Value: "codex:gpt-5.5"},
		{Agent: "echo", Label: "Echo", Model: "", Value: "echo:default"},
	}, options)
}
