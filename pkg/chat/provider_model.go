package chat

import (
	"strings"

	"github.com/housecat-inc/scratch/uikit"
)

func ParseProviderModel(value string) (string, string) {
	agent, model, ok := strings.Cut(strings.TrimSpace(value), ":")
	if !ok {
		return strings.TrimSpace(value), ""
	}
	if model == "default" {
		model = ""
	}
	return strings.TrimSpace(agent), strings.TrimSpace(model)
}

func ProviderModelOptions(agents []string) []uikit.SelectOption {
	options := []uikit.SelectOption{}
	for _, agent := range agents {
		agent = strings.TrimSpace(agent)
		if agent == "" || agent == "contact" {
			continue
		}
		switch agent {
		case "claude":
			options = append(options,
				providerModelOption("claude", "", "Claude Default"),
				providerModelOption("claude", "haiku", "Claude Haiku"),
				providerModelOption("claude", "opus", "Claude Opus"),
				providerModelOption("claude", "sonnet", "Claude Sonnet"),
			)
		case "codex":
			options = append(options,
				providerModelOption("codex", "", "Codex Default"),
				providerModelOption("codex", "gpt-5", "Codex GPT 5"),
				providerModelOption("codex", "gpt-5.1", "Codex GPT 5.1"),
				providerModelOption("codex", "gpt-5.5", "Codex GPT 5.5"),
			)
		case "echo":
			options = append(options, providerModelOption("echo", "", "Echo"))
		default:
			options = append(options, providerModelOption(agent, "", agent))
		}
	}
	return options
}

func NormalizeModel(agent, model string) string {
	model = strings.TrimSpace(model)
	if model == "" || model == "default" {
		return ""
	}
	switch agent {
	case "claude":
		switch model {
		case "haiku", "opus", "sonnet":
			return model
		}
	case "codex":
		switch model {
		case "gpt-5", "gpt-5.1", "gpt-5.5":
			return model
		}
	}
	return ""
}

func providerModelOption(agent, model, label string) uikit.SelectOption {
	valueModel := model
	if valueModel == "" {
		valueModel = "default"
	}
	return uikit.SelectOption{
		Label: label,
		Value: agent + ":" + valueModel,
	}
}
