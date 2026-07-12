package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"

	"github.com/cockroachdb/errors"
)

type CodexAgent struct {
	Dir string
}

type codexAnchor struct {
	Agent    string `json:"agent"`
	Model    string `json:"model,omitempty"`
	ThreadID string `json:"thread_id,omitempty"`
}

type codexItem struct {
	Input json.RawMessage `json:"input"`
	Name  string          `json:"name"`
	Text  string          `json:"text"`
	Type  string          `json:"type"`
}

type codexLine struct {
	Item     codexItem `json:"item"`
	ThreadID string    `json:"thread_id"`
	Type     string    `json:"type"`
}

func (CodexAgent) Author() string { return "agent:codex" }

func (a CodexAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	var state codexAnchor
	_ = json.Unmarshal([]byte(turn.Anchor), &state)

	args := []string{"exec", "--json", "--sandbox", "workspace-write", "--skip-git-repo-check"}
	if state.Model != "" {
		args = append(args, "--model", state.Model)
	}
	args = append(args, turn.Prompt)
	if state.ThreadID != "" {
		args = []string{"exec", "resume", "--json", "--skip-git-repo-check"}
		if state.Model != "" {
			args = append(args, "--model", state.Model)
		}
		args = append(args, state.ThreadID, turn.Prompt)
	}
	cmd := exec.CommandContext(ctx, "codex", args...)
	cmd.Dir = a.Dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", errors.Wrap(err, "stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return "", errors.Wrap(err, "start codex")
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		var line codexLine
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		if line.ThreadID != "" {
			state.ThreadID = line.ThreadID
		}
		switch line.Type {
		case "item.completed":
			switch line.Item.Type {
			case "agent_message":
				if line.Item.Text != "" {
					emit(DeltaEvent(line.Item.Text))
				}
			case "tool_call":
				data, _ := json.Marshal(map[string]any{"input": line.Item.Input, "name": line.Item.Name})
				emit(Event{Data: string(data), Type: EventToolCall})
			}
		case "turn.completed":
			emit(Event{Data: string(scanner.Bytes()), Type: EventResult})
		}
	}
	if err := scanner.Err(); err != nil {
		cmd.Wait()
		return "", errors.Wrap(err, "read codex output")
	}
	if err := cmd.Wait(); err != nil {
		return "", errors.Wrapf(err, "codex: %s", bytes.TrimSpace(stderr.Bytes()))
	}
	state.Agent = "codex"
	out, err := json.Marshal(state)
	if err != nil {
		return "", errors.Wrap(err, "marshal anchor")
	}
	return string(out), nil
}
