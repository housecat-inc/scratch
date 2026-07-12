package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

type CodexAgent struct {
	Dir string
}

type codexAnchor struct {
	Access   string `json:"access,omitempty"`
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

func (a CodexAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	var state codexAnchor
	_ = json.Unmarshal([]byte(turn.Anchor), &state)

	parser := &codexParser{state: &state}
	if err := runTurn(ctx, turn, emit, "codex", codexArgs(state, turn, a.Dir), a.Dir, parser); err != nil {
		return "", err
	}
	state.Agent = "codex"
	out, err := json.Marshal(state)
	if err != nil {
		return "", errors.Wrap(err, "marshal anchor")
	}
	return string(out), nil
}

func codexArgs(state codexAnchor, turn Turn, dir string) []string {
	sandbox := "workspace-write"
	if state.Access == AccessSafe {
		sandbox = "read-only"
	}
	args := []string{"exec"}
	if state.ThreadID != "" {
		args = append(args, "resume")
	}
	args = append(args, "--json", "--skip-git-repo-check")
	if state.ThreadID == "" {
		args = append(args, "--sandbox", sandbox)
	} else {
		args = append(args, "-c", fmt.Sprintf("sandbox_mode=%q", sandbox))
	}
	if sandbox == "workspace-write" {
		if gitDir := worktreeGitDir(dir); gitDir != "" {
			args = append(args, "-c", fmt.Sprintf("sandbox_workspace_write.writable_roots=[%q]", gitDir))
		}
	}
	if state.Model != "" {
		args = append(args, "--model", state.Model)
	}
	docs := []Attachment{}
	for _, f := range turn.Attachments {
		if strings.HasPrefix(f.MimeType, "image/") {
			args = append(args, "--image="+f.Path)
		} else {
			docs = append(docs, f)
		}
	}
	if state.ThreadID != "" {
		args = append(args, state.ThreadID)
	}
	return append(args, turn.Prompt+attachmentAppendix(docs))
}

func worktreeGitDir(dir string) string {
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return ""
		}
		dir = cwd
	}
	data, err := os.ReadFile(filepath.Join(dir, ".git"))
	if err != nil {
		return ""
	}
	gitdir := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(string(data)), "gitdir:"))
	if gitdir == "" {
		return ""
	}
	if !filepath.IsAbs(gitdir) {
		gitdir = filepath.Join(dir, gitdir)
	}
	gitdir = filepath.Clean(gitdir)
	if i := strings.Index(gitdir, string(filepath.Separator)+".git"+string(filepath.Separator)); i >= 0 {
		return gitdir[:i+len("/.git")]
	}
	return gitdir
}

type codexParser struct {
	completed   bool
	result      error
	state       *codexAnchor
	textEmitted bool
}

func (p *codexParser) Completed() bool { return p.completed }

func (p *codexParser) Result() error { return p.result }

func (p *codexParser) Parse(raw []byte) []Event {
	var line codexLine
	if err := json.Unmarshal(raw, &line); err != nil {
		return nil
	}
	events := []Event{}
	if line.ThreadID != "" && line.ThreadID != p.state.ThreadID {
		p.state.Agent = "codex"
		p.state.ThreadID = line.ThreadID
		if data, err := json.Marshal(p.state); err == nil {
			events = append(events, Event{Data: string(data), Type: EventAnchor})
		}
	}
	switch line.Type {
	case "item.started", "item.updated", "item.completed":
		return append(events, codexItemEvents(line.Type, line.Item, &p.textEmitted)...)
	case "turn.completed":
		p.completed = true
		return append(events, Event{Data: string(raw), Type: EventResult})
	case "turn.failed", "error":
		message := line.Error.Message
		if message == "" {
			message = line.Message
		}
		p.completed = true
		p.result = errors.Newf("codex: %s", message)
	}
	return events
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
