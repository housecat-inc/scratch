package chat

import (
	"encoding/json"
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
)

const (
	PartPlan     = "plan"
	PartText     = "text"
	PartThinking = "thinking"
	PartTool     = "tool"

	ToolStatusCompleted  = "completed"
	ToolStatusFailed     = "failed"
	ToolStatusInProgress = "in_progress"
)

type MessagePart struct {
	Kind string
	Plan []PlanEntry
	Text string
	Tool *ToolCallView
}

type PlanEntry struct {
	Content  string `json:"content"`
	Priority string `json:"priority,omitempty"`
	Status   string `json:"status,omitempty"`
}

type ToolCallView struct {
	Detail string
	ID     string
	Status string
	Title  string
}

type toolCallData struct {
	Content    json.RawMessage `json:"content"`
	Input      json.RawMessage `json:"input"`
	Name       string          `json:"name"`
	Status     string          `json:"status"`
	Title      string          `json:"title"`
	ToolCallID string          `json:"toolCallId"`
}

type toolContentItem struct {
	Content struct {
		Text string `json:"text"`
	} `json:"content"`
	NewText string `json:"newText"`
	OldText string `json:"oldText"`
	Path    string `json:"path"`
	Type    string `json:"type"`
}

func foldParts(events []db.MessageEvent, streaming bool) []MessagePart {
	parts := []MessagePart{}
	planIndex := -1
	tools := map[string]*ToolCallView{}

	appendText := func(kind, text string) {
		if text == "" {
			return
		}
		if n := len(parts) - 1; n >= 0 && parts[n].Kind == kind {
			parts[n].Text += text
			return
		}
		parts = append(parts, MessagePart{Kind: kind, Text: text})
	}

	for _, ev := range events {
		switch ev.Type {
		case EventDelta, EventThinking:
			var data struct {
				Text string `json:"text"`
			}
			if json.Unmarshal([]byte(ev.Data), &data) != nil {
				continue
			}
			kind := PartText
			if ev.Type == EventThinking {
				kind = PartThinking
			}
			appendText(kind, data.Text)
		case EventPlan:
			var data struct {
				Entries []PlanEntry `json:"entries"`
			}
			if json.Unmarshal([]byte(ev.Data), &data) != nil || len(data.Entries) == 0 {
				continue
			}
			if planIndex >= 0 {
				parts[planIndex].Plan = data.Entries
				continue
			}
			planIndex = len(parts)
			parts = append(parts, MessagePart{Kind: PartPlan, Plan: data.Entries})
		case EventToolCall, EventToolCallUpdate:
			var data toolCallData
			if json.Unmarshal([]byte(ev.Data), &data) != nil {
				continue
			}
			if tool, ok := tools[data.ToolCallID]; ok && data.ToolCallID != "" {
				mergeToolCall(tool, data)
				continue
			}
			tool := &ToolCallView{ID: data.ToolCallID, Status: ToolStatusInProgress}
			if ev.Type == EventToolCall && data.ToolCallID == "" {
				tool.Status = ToolStatusCompleted
			}
			mergeToolCall(tool, data)
			if data.ToolCallID != "" {
				tools[data.ToolCallID] = tool
			}
			parts = append(parts, MessagePart{Kind: PartTool, Tool: tool})
		}
	}

	if !streaming {
		for _, tool := range tools {
			if tool.Status == ToolStatusInProgress {
				tool.Status = ToolStatusCompleted
			}
		}
	}
	return parts
}

func mergeToolCall(tool *ToolCallView, data toolCallData) {
	if detail := toolDetail(data); detail != "" {
		tool.Detail = detail
	}
	if data.Status != "" {
		tool.Status = data.Status
	}
	switch {
	case data.Title != "":
		tool.Title = data.Title
	case data.Name != "" && tool.Title == "":
		tool.Title = data.Name
	}
}

func toolDetail(data toolCallData) string {
	if len(data.Content) > 0 {
		var items []toolContentItem
		if json.Unmarshal(data.Content, &items) == nil {
			lines := []string{}
			for _, item := range items {
				switch item.Type {
				case "diff":
					lines = append(lines, renderDiff(item))
				default:
					if item.Content.Text != "" {
						lines = append(lines, item.Content.Text)
					}
				}
			}
			if joined := strings.TrimSpace(strings.Join(lines, "\n")); joined != "" {
				return joined
			}
		}
	}
	if len(data.Input) > 0 {
		var pretty map[string]any
		if json.Unmarshal(data.Input, &pretty) == nil {
			if out, err := json.MarshalIndent(pretty, "", "  "); err == nil {
				return string(out)
			}
		}
		return string(data.Input)
	}
	return ""
}

func renderDiff(item toolContentItem) string {
	lines := []string{}
	if item.Path != "" {
		lines = append(lines, item.Path)
	}
	for _, line := range splitDiffLines(item.OldText) {
		lines = append(lines, "- "+line)
	}
	for _, line := range splitDiffLines(item.NewText) {
		lines = append(lines, "+ "+line)
	}
	return strings.Join(lines, "\n")
}

func splitDiffLines(text string) []string {
	if text == "" {
		return nil
	}
	return strings.Split(strings.TrimRight(text, "\n"), "\n")
}
