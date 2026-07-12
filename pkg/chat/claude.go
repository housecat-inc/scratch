package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/cockroachdb/errors"
)

type ClaudeAgent struct {
	Dir string
}

type claudeAnchor struct {
	Agent     string `json:"agent"`
	SessionID string `json:"session_id,omitempty"`
}

type claudeLine struct {
	IsError bool   `json:"is_error"`
	Message struct {
		Content []struct {
			Input json.RawMessage `json:"input"`
			Name  string          `json:"name"`
			Text  string          `json:"text"`
			Type  string          `json:"type"`
		} `json:"content"`
	} `json:"message"`
	Result    string `json:"result"`
	SessionID string `json:"session_id"`
	Subtype   string `json:"subtype"`
	Type      string `json:"type"`
}

func (ClaudeAgent) Author() string { return "agent:claude" }

func (a ClaudeAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	var state claudeAnchor
	_ = json.Unmarshal([]byte(turn.Anchor), &state)

	args := []string{"-p", turn.Prompt, "--output-format", "stream-json", "--verbose"}
	if state.SessionID != "" {
		args = append(args, "--resume", state.SessionID)
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = a.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", errors.Wrap(err, "stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return "", errors.Wrap(err, "start claude")
	}

	texts := 0
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var line claudeLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.SessionID != "" {
			state.SessionID = line.SessionID
		}
		switch line.Type {
		case "assistant":
			for _, block := range line.Message.Content {
				switch block.Type {
				case "text":
					text := block.Text
					if texts > 0 {
						text = "\n\n" + text
					}
					texts++
					emit(DeltaEvent(text))
				case "tool_use":
					data, _ := json.Marshal(map[string]any{"input": block.Input, "name": block.Name})
					emit(Event{Data: string(data), Type: EventToolCall})
				}
			}
		case "result":
			emit(Event{Data: string(scanner.Bytes()), Type: EventResult})
			if line.IsError && line.Result != "" {
				cmd.Wait()
				return "", errors.Newf("claude: %s", line.Result)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		cmd.Wait()
		return "", errors.Wrap(err, "read claude output")
	}
	if err := cmd.Wait(); err != nil {
		return "", errors.Wrapf(err, "claude: %s", bytes.TrimSpace(stderr.Bytes()))
	}
	state.Agent = "claude"
	out, err := json.Marshal(state)
	if err != nil {
		return "", errors.Wrap(err, "marshal anchor")
	}
	return string(out), nil
}
