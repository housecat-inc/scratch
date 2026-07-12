package chat

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"

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
	AggregatedOutput string `json:"aggregated_output"`
	Changes          []struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"changes"`
	Command  string `json:"command"`
	ExitCode *int   `json:"exit_code"`
	ID       string `json:"id"`
	Items    []struct {
		Completed bool   `json:"completed"`
		Text      string `json:"text"`
	} `json:"items"`
	Query  string `json:"query"`
	Server string `json:"server"`
	Text   string `json:"text"`
	Tool   string `json:"tool"`
	Type   string `json:"type"`
}

type codexLine struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
	Item     codexItem `json:"item"`
	Message  string    `json:"message"`
	ThreadID string    `json:"thread_id"`
	Type     string    `json:"type"`
}

func (CodexAgent) Author() string { return "agent:codex" }

func codexArgs(state codexAnchor, prompt string) []string {
	args := []string{"exec"}
	if state.ThreadID != "" {
		args = append(args, "resume")
	}
	args = append(args, "--json", "--skip-git-repo-check")
	if state.ThreadID == "" {
		args = append(args, "--sandbox", "workspace-write")
	} else {
		args = append(args, "-c", `sandbox_mode="workspace-write"`)
	}
	if state.Model != "" {
		args = append(args, "--model", state.Model)
	}
	if state.ThreadID != "" {
		args = append(args, state.ThreadID)
	}
	return append(args, prompt)
}

func (a CodexAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	var state codexAnchor
	_ = json.Unmarshal([]byte(turn.Anchor), &state)

	cmd := exec.CommandContext(ctx, "codex", codexArgs(state, turn.Prompt)...)
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

	textEmitted := false
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
		case "item.started", "item.updated", "item.completed":
			for _, ev := range codexItemEvents(line.Type, line.Item, &textEmitted) {
				emit(ev)
			}
		case "turn.completed":
			emit(Event{Data: string(scanner.Bytes()), Type: EventResult})
		case "turn.failed", "error":
			message := line.Error.Message
			if message == "" {
				message = line.Message
			}
			cmd.Wait()
			return "", errors.Newf("codex: %s", message)
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

func codexItemEvents(lineType string, item codexItem, textEmitted *bool) []Event {
	completed := lineType == "item.completed"
	switch item.Type {
	case "agent_message":
		if !completed || item.Text == "" {
			return nil
		}
		text := item.Text
		if *textEmitted {
			text = "\n\n" + text
		}
		*textEmitted = true
		return []Event{DeltaEvent(text)}
	case "reasoning":
		if !completed || item.Text == "" {
			return nil
		}
		return []Event{thinkingEvent(item.Text + "\n\n")}
	case "command_execution":
		if lineType == "item.updated" {
			return nil
		}
		if !completed {
			return toolEvents(EventToolCall, map[string]any{
				"status":     ToolStatusInProgress,
				"title":      clip(item.Command, 120),
				"toolCallId": item.ID,
			})
		}
		status := ToolStatusCompleted
		if item.ExitCode != nil && *item.ExitCode != 0 {
			status = ToolStatusFailed
		}
		payload := map[string]any{
			"status":     status,
			"title":      clip(item.Command, 120),
			"toolCallId": item.ID,
		}
		if out := clip(item.AggregatedOutput, 8192); out != "" {
			payload["content"] = []map[string]any{
				{"content": map[string]string{"text": out, "type": "text"}, "type": "content"},
			}
		}
		return toolEvents(EventToolCallUpdate, payload)
	case "file_change":
		if !completed {
			return nil
		}
		paths := make([]string, 0, len(item.Changes))
		for _, change := range item.Changes {
			paths = append(paths, change.Kind+" "+change.Path)
		}
		return toolEvents(EventToolCall, map[string]any{
			"status":     ToolStatusCompleted,
			"title":      clip("Edit: "+strings.Join(paths, ", "), 120),
			"toolCallId": item.ID,
		})
	case "mcp_tool_call":
		if !completed {
			return nil
		}
		return toolEvents(EventToolCall, map[string]any{
			"status":     ToolStatusCompleted,
			"title":      clip(item.Server+"."+item.Tool, 120),
			"toolCallId": item.ID,
		})
	case "todo_list":
		entries := make([]PlanEntry, 0, len(item.Items))
		for _, todo := range item.Items {
			status := "pending"
			if todo.Completed {
				status = "completed"
			}
			entries = append(entries, PlanEntry{Content: todo.Text, Status: status})
		}
		if len(entries) == 0 {
			return nil
		}
		data, err := json.Marshal(map[string]any{"entries": entries})
		if err != nil {
			return nil
		}
		return []Event{{Data: string(data), Type: EventPlan}}
	case "web_search":
		if !completed {
			return nil
		}
		return toolEvents(EventToolCall, map[string]any{
			"status":     ToolStatusCompleted,
			"title":      clip("Search: "+item.Query, 120),
			"toolCallId": item.ID,
		})
	}
	return nil
}

func toolEvents(eventType string, payload map[string]any) []Event {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return []Event{{Data: string(data), Type: eventType}}
}
