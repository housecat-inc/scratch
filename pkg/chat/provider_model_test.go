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
		in    string
		model string
		name  string
	}{
		{agent: "codex", in: "codex:default", name: "drops the default model"},
		{agent: "codex", in: "codex:gpt-5.5", model: "gpt-5.5", name: "splits the agent and model"},
		{agent: "echo", in: "echo", name: "accepts an agent without a model"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)

			agent, model := ParseProviderModel(tt.in)
			a.Equal(tt.agent, agent)
			a.Equal(tt.model, model)
		})
	}
}

func TestProviderModelOptions(t *testing.T) {
	t.Run("lists each agent's models and hides the contact agent", func(t *testing.T) {
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
	})
}
