package chat

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
)

const (
	EventAnchor            = "anchor"
	EventDelta             = "delta"
	EventElicitation       = "elicitation"
	EventElicitationResult = "elicitation_result"
	EventError             = "error"
	EventPlan              = "plan"
	EventResult            = "result"
	EventRunner            = "runner"
	EventThinking          = "thinking"
	EventToolCall          = "tool_call"
	EventToolCallUpdate    = "tool_call_update"
	EventUsage             = "usage"
)

type Attachment struct {
	MimeType string
	Path     string
}

type Event struct {
	Data string
	Type string
}

type Turn struct {
	Anchor      string
	Attachments []Attachment
	MessageID   int64
	Meta        string
	Prompt      string
	ThreadID    int64
}

type Agent interface {
	Author() string
	Run(ctx context.Context, turn Turn, emit func(Event)) (string, error)
}

func DeltaEvent(text string) Event {
	data, _ := json.Marshal(map[string]string{"text": text})
	return Event{Data: string(data), Type: EventDelta}
}

func thinkingEvent(text string) Event {
	data, _ := json.Marshal(map[string]string{"text": text})
	return Event{Data: string(data), Type: EventThinking}
}

func attachmentAppendix(files []Attachment) string {
	if len(files) == 0 {
		return ""
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, "- "+f.Path)
	}
	return "\n\nAttached files:\n" + strings.Join(paths, "\n")
}

func clip(s string, n int) string {
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + "…"
}

func toolTitle(name string, input json.RawMessage) string {
	var params map[string]any
	_ = json.Unmarshal(input, &params)
	for _, key := range []string{"command", "file_path", "path", "pattern", "query", "url", "description", "prompt"} {
		if value, ok := params[key].(string); ok && value != "" {
			return clip(name+" "+value, 120)
		}
	}
	return name
}

func ResolveAgent(name string) (Agent, error) {
	switch name {
	case "", "auto":
		if _, err := exec.LookPath("codex"); err == nil {
			return CodexAgent{}, nil
		}
		if _, err := exec.LookPath("claude"); err == nil {
			return ClaudeAgent{}, nil
		}
		return EchoAgent{Delay: 200 * time.Millisecond}, nil
	case "claude":
		return ClaudeAgent{}, nil
	case "codex":
		return CodexAgent{}, nil
	case "echo":
		return EchoAgent{Delay: 200 * time.Millisecond}, nil
	}
	return nil, errors.Newf("unknown agent %q", name)
}

func ResolveAgentInDir(name, dir string) (Agent, error) {
	agent, err := ResolveAgent(name)
	if err != nil {
		return nil, err
	}
	return WithDir(agent, dir), nil
}

func AvailableAgents() []Agent {
	agents := []Agent{}
	if _, err := exec.LookPath("codex"); err == nil {
		agents = append(agents, CodexAgent{})
	}
	if _, err := exec.LookPath("claude"); err == nil {
		agents = append(agents, ClaudeAgent{})
	}
	agents = append(agents, EchoAgent{Delay: 200 * time.Millisecond})
	return agents
}

func AvailableAgentsInDir(dir string) []Agent {
	agents := AvailableAgents()
	for i, agent := range agents {
		agents[i] = WithDir(agent, dir)
	}
	return agents
}

func WithDir(agent Agent, dir string) Agent {
	if dir == "" {
		return agent
	}
	switch a := agent.(type) {
	case ClaudeAgent:
		a.Dir = dir
		return a
	case CodexAgent:
		a.Dir = dir
		return a
	default:
		return agent
	}
}

type EchoAgent struct {
	Delay time.Duration
}

func (EchoAgent) Author() string { return "agent:echo" }

func (a EchoAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	for _, chunk := range []string{"You said: ", turn.Prompt} {
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
