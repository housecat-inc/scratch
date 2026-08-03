package chat

import (
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/housecat-inc/scratch/uikit"
	"github.com/stretchr/testify/assert"
)

func TestParseProviderModel(t *testing.T) {
	t.Parallel()

	type out struct{ agent, model string }

	tk.Run(t, []tk.Test[string, out]{
		{Name: "default", In: "codex:default", Out: out{agent: "codex"}},
		{Name: "model", In: "codex:gpt-5.5", Out: out{agent: "codex", model: "gpt-5.5"}},
		{Name: "agent only", In: "echo", Out: out{agent: "echo"}},
	}, tk.Pure(func(value string) out {
		agent, model := ParseProviderModel(value)
		return out{agent: agent, model: model}
	}), tk.Parallel())
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
