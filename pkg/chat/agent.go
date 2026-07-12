package chat

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	EventDelta    = "delta"
	EventError    = "error"
	EventResult   = "result"
	EventToolCall = "tool_call"
)

type Event struct {
	Data string
	Type string
}

type Agent interface {
	Author() string
	Run(ctx context.Context, anchor, prompt string, emit func(Event)) (string, error)
}

func DeltaEvent(text string) Event {
	data, _ := json.Marshal(map[string]string{"text": text})
	return Event{Data: string(data), Type: EventDelta}
}

func ResolveAgent(name string) (Agent, error) {
	switch name {
	case "", "auto":
		if _, err := exec.LookPath("claude"); err == nil {
			return ClaudeAgent{}, nil
		}
		return EchoAgent{Delay: 200 * time.Millisecond}, nil
	case "claude":
		return ClaudeAgent{}, nil
	case "echo":
		return EchoAgent{Delay: 200 * time.Millisecond}, nil
	}
	return nil, errors.Newf("unknown agent %q", name)
}

type EchoAgent struct {
	Delay time.Duration
}

func (EchoAgent) Author() string { return "agent:echo" }

func (a EchoAgent) Run(ctx context.Context, anchor, prompt string, emit func(Event)) (string, error) {
	for _, chunk := range []string{"You said: ", prompt} {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(a.Delay):
		}
		emit(DeltaEvent(chunk))
	}
	emit(Event{Data: "{}", Type: EventResult})
	return "", nil
}
