package codex

import (
	"context"
	"strings"
)

const AutomatedNoticePrefix = "Automated notice, not a message from the user: "

type (
	AutomatedLoader    func(context.Context, string) (map[string]bool, error)
	automatedLoaderKey struct{}
)

func WithAutomatedLoader(ctx context.Context, load AutomatedLoader) context.Context {
	return context.WithValue(ctx, automatedLoaderKey{}, load)
}

func automatedPrompt(ctx context.Context, prompt ChatItem) (string, bool) {
	in, _ := ctx.Value(linksKey{}).(ChatIn)
	if prompt.ClientID == "" || !in.Automated[prompt.ClientID] {
		return prompt.Text, false
	}
	return strings.TrimPrefix(prompt.Text, AutomatedNoticePrefix), true
}
