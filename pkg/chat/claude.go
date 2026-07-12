package chat

import (
	"context"
	"encoding/json"

	"github.com/cockroachdb/errors"
)

type ClaudeAgent struct {
	Dir string
}

type claudeAnchor struct {
	Access    string `json:"access,omitempty"`
	Agent     string `json:"agent"`
	Model     string `json:"model,omitempty"`
	SessionID string `json:"session_id,omitempty"`
}

type claudeBlock struct {
	Content   json.RawMessage `json:"content"`
	ID        string          `json:"id"`
	Input     json.RawMessage `json:"input"`
	IsError   bool            `json:"is_error"`
	Name      string          `json:"name"`
	Text      string          `json:"text"`
	Thinking  string          `json:"thinking"`
	ToolUseID string          `json:"tool_use_id"`
	Type      string          `json:"type"`
}

type claudeLine struct {
	Event struct {
		ContentBlock struct {
			Type string `json:"type"`
		} `json:"content_block"`
		Delta struct {
			Text     string `json:"text"`
			Thinking string `json:"thinking"`
			Type     string `json:"type"`
		} `json:"delta"`
		Type string `json:"type"`
	} `json:"event"`
	IsError         bool            `json:"is_error"`
	Message         json.RawMessage `json:"message"`
	ParentToolUseID *string         `json:"parent_tool_use_id"`
	Result          string          `json:"result"`
	SessionID       string          `json:"session_id"`
	Type            string          `json:"type"`
}

func (ClaudeAgent) Author() string { return "agent:claude" }

func (a ClaudeAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	var state claudeAnchor
	_ = json.Unmarshal([]byte(turn.Anchor), &state)

	parser := &claudeParser{state: &state}
	if err := runTurn(ctx, turn, emit, "claude", claudeArgs(state, turn, a.Dir), a.Dir, parser); err != nil {
		return "", err
	}
	state.Agent = "claude"
	out, err := json.Marshal(state)
	if err != nil {
		return "", errors.Wrap(err, "marshal anchor")
	}
	return string(out), nil
}

func claudeArgs(state claudeAnchor, turn Turn, dir string) []string {
	mode := "bypassPermissions"
	if state.Access == AccessSafe {
		mode = "default"
	}
	args := []string{
		"-p", turn.Prompt + attachmentAppendix(turn.Attachments),
		"--append-system-prompt", agentInstructions(dir),
		"--include-partial-messages",
		"--output-format", "stream-json",
		"--permission-mode", mode,
		"--verbose",
	}
	if state.Model != "" {
		args = append(args, "--model", state.Model)
	}
	if state.SessionID != "" {
		args = append(args, "--resume", state.SessionID)
	}
	return args
}

type claudeParser struct {
	completed   bool
	result      error
	state       *claudeAnchor
	textEmitted bool
}

func (p *claudeParser) Completed() bool { return p.completed }

func (p *claudeParser) Result() error { return p.result }

func (p *claudeParser) Parse(raw []byte) []Event {
	var line claudeLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return nil
	}
	events := []Event{}
	if line.SessionID != "" && line.SessionID != p.state.SessionID {
		p.state.Agent = "claude"
		p.state.SessionID = line.SessionID
		if data, err := json.Marshal(p.state); err == nil {
			events = append(events, Event{Data: string(data), Type: EventAnchor})
		}
	}
	if line.Type == "result" {
		p.completed = true
		if line.IsError && line.Result != "" {
			p.result = errors.Newf("claude: %s", line.Result)
		}
	}
	return append(events, claudeLineEvents(line, raw, &p.textEmitted)...)
}

func claudeLineEvents(line claudeLine, raw []byte, textEmitted *bool) []Event {
	if line.ParentToolUseID != nil {
		return nil
	}
	switch line.Type {
	case "stream_event":
		switch line.Event.Type {
		case "content_block_start":
			if line.Event.ContentBlock.Type == "text" && *textEmitted {
				return []Event{DeltaEvent("\n\n")}
			}
		case "content_block_delta":
			switch line.Event.Delta.Type {
			case "text_delta":
				if line.Event.Delta.Text != "" {
					*textEmitted = true
					return []Event{DeltaEvent(line.Event.Delta.Text)}
				}
			case "thinking_delta":
				if line.Event.Delta.Thinking != "" {
					return []Event{thinkingEvent(line.Event.Delta.Thinking)}
				}
			}
		}
	case "assistant":
		events := []Event{}
		for _, block := range claudeBlocks(line.Message) {
			if block.Type != "tool_use" {
				continue
			}
			events = append(events, toolEvents(EventToolCall, map[string]any{
				"input":      block.Input,
				"status":     ToolStatusInProgress,
				"title":      toolTitle(block.Name, block.Input),
				"toolCallId": block.ID,
			})...)
		}
		return events
	case "user":
		events := []Event{}
		for _, block := range claudeBlocks(line.Message) {
			if block.Type != "tool_result" {
				continue
			}
			status := ToolStatusCompleted
			if block.IsError {
				status = ToolStatusFailed
			}
			payload := map[string]any{
				"status":     status,
				"toolCallId": block.ToolUseID,
			}
			if text := clip(toolResultText(block.Content), 8192); text != "" {
				payload["content"] = []map[string]any{
					{"content": map[string]string{"text": text, "type": "text"}, "type": "content"},
				}
			}
			events = append(events, toolEvents(EventToolCallUpdate, payload)...)
		}
		return events
	case "result":
		return []Event{{Data: string(raw), Type: EventResult}}
	}
	return nil
}

func claudeBlocks(message json.RawMessage) []claudeBlock {
	var payload struct {
		Content []claudeBlock `json:"content"`
	}
	if json.Unmarshal(message, &payload) != nil {
		return nil
	}
	return payload.Content
}

func toolResultText(content json.RawMessage) string {
	if len(content) == 0 {
		return ""
	}
	var text string
	if json.Unmarshal(content, &text) == nil {
		return text
	}
	var blocks []claudeBlock
	if json.Unmarshal(content, &blocks) != nil {
		return ""
	}
	out := ""
	for _, block := range blocks {
		if block.Type == "text" && block.Text != "" {
			if out != "" {
				out += "\n"
			}
			out += block.Text
		}
	}
	return out
}
