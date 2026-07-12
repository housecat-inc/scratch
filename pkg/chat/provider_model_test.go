package chat

import (
	"testing"

	"github.com/housecat-inc/scratch/uikit"
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
	a.Equal([]uikit.SelectOption{
		{Label: "Claude Default", Value: "claude:default"},
		{Label: "Claude Haiku", Value: "claude:haiku"},
		{Label: "Claude Opus", Value: "claude:opus"},
		{Label: "Claude Sonnet", Value: "claude:sonnet"},
		{Label: "Codex Default", Value: "codex:default"},
		{Label: "Codex GPT 5", Value: "codex:gpt-5"},
		{Label: "Codex GPT 5.1", Value: "codex:gpt-5.1"},
		{Label: "Codex GPT 5.5", Value: "codex:gpt-5.5"},
		{Label: "Echo", Value: "echo:default"},
	}, options)
}
