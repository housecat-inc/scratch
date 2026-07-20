package ui

import (
	_ "embed"
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

//go:embed static/chat.css
var chatCSS string

type ChatFormFieldProps struct {
	Label      string
	Name       string
	Options    []string
	Required   bool
	Type       string
	Value      string
	ValueLabel string
}

type ChatFormProps struct {
	AcceptLabel   string
	Action        string
	DeclineLabel  string
	Disabled      bool
	Editable      bool
	ElicitationID string
	Fields        []ChatFormFieldProps
	ForkAction    string
	HideMessage   bool
	Message       string
	MessageID     int64
	Plain         bool
}

func acceptLabel(f ChatFormProps) string {
	if f.AcceptLabel != "" {
		return f.AcceptLabel
	}
	return "Accept"
}

func declineLabel(f ChatFormProps) string {
	if f.DeclineLabel != "" {
		return f.DeclineLabel
	}
	return "Decline"
}

type ChatAttachmentProps struct {
	ID       int64
	IsImage  bool
	Name     string
	ThreadID int64
}

type ChatComposerProps struct {
	Action      string
	Agent       string
	FileName    string
	Hidden      []ChatComposerHiddenProps
	HXPost      string
	IDPrefix    string
	Model       string
	Placeholder string
	StopAction  string
	UploadURL   string
}

type ChatComposerHiddenProps struct {
	Name  string
	Value string
}

type ChatMessageProps struct {
	Attachments []ChatAttachmentProps
	Author      string
	Body        string
	Form        *ChatFormProps
	ID          int64
	Parts       []ChatPartProps
	Role        string
	Status      string
}

type ChatPartProps struct {
	Kind    string
	Plan    []ChatPlanEntryProps
	Summary string
	Text    string
	Tool    *ChatToolCallProps
}

type chatRowProps struct {
	Detail  string
	Failed  bool
	Key     string
	Kind    string
	Label   string
	Live    bool
	Shimmer bool
	Summary string
}

type ChatPlanEntryProps struct {
	Content string
	Status  string
}

type ChatToolCallProps struct {
	Detail  string
	ID      string
	Status  string
	Summary string
	Title   string
}

type chatPartGroup struct {
	Key     string
	Kind    string
	Last    bool
	Part    ChatPartProps
	Summary string
	Tools   []ChatPartProps
}

type ChatThreadProps struct {
	Agent    string
	ID       int64
	Messages []ChatMessageProps
	Title    string
}

type FloatingChatProps struct {
	Access      string
	Agent       string
	Description string
	ID          int64
	Messages    []ChatMessageProps
	Streaming   bool
	Title       string
}

func ChatCSS() templ.Component {
	return templ.Raw(`<style>` + chatCSS + `</style>`)
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func chatDOMID(p ChatComposerProps, id string) string {
	if p.IDPrefix == "" {
		return id
	}
	return p.IDPrefix + "-" + id
}

func accessLabel(access string) string {
	if access == "full" {
		return "Full access — agent can edit files and run commands"
	}
	return "Safe mode — agent is read-only"
}

func attachmentURL(a ChatAttachmentProps) string {
	return "/chat/" + itoa64(a.ThreadID) + "/attachments/" + itoa64(a.ID)
}

func chatPartGroups(parts []ChatPartProps) []chatPartGroup {
	groups := []chatPartGroup{}
	for i := 0; i < len(parts); {
		part := parts[i]
		if part.Kind != "tool" {
			groups = append(groups, chatPartGroup{Kind: part.Kind, Last: i == len(parts)-1, Part: part})
			i++
			continue
		}

		j := i
		for j < len(parts) && parts[j].Kind == "tool" {
			j++
		}
		tools := parts[i:j]
		if len(tools) == 1 {
			groups = append(groups, chatPartGroup{Kind: part.Kind, Last: j == len(parts), Part: part})
		} else {
			groups = append(groups, chatPartGroup{
				Key:     "tool-group-" + strconv.Itoa(i),
				Kind:    "tools",
				Last:    j == len(parts),
				Summary: chatToolGroupSummary(tools),
				Tools:   tools,
			})
		}
		i = j
	}
	return groups
}

func chatToolGroupFailed(tools []ChatPartProps) bool {
	for _, part := range tools {
		if part.Tool != nil && part.Tool.Status == "failed" {
			return true
		}
	}
	return false
}

func chatToolGroupLabel(tools []ChatPartProps) string {
	return plural(len(tools), "tool call")
}

func chatToolGroupLive(tools []ChatPartProps) bool {
	for _, part := range tools {
		if part.Tool != nil && (part.Tool.Status == "in_progress" || part.Tool.Status == "pending") {
			return true
		}
	}
	return false
}

func chatToolGroupSummary(tools []ChatPartProps) string {
	completed := 0
	failed := 0
	running := 0
	for _, part := range tools {
		if part.Tool == nil {
			continue
		}
		switch part.Tool.Status {
		case "failed":
			failed++
		case "in_progress", "pending":
			running++
		default:
			completed++
		}
	}

	parts := []string{}
	if running > 0 {
		parts = append(parts, pluralStatus(running, "running"))
	}
	if failed > 0 {
		parts = append(parts, pluralStatus(failed, "failed"))
	}
	if completed > 0 {
		parts = append(parts, pluralStatus(completed, "completed"))
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, ", ")
}

func planStatusIcon(status string) string {
	switch status {
	case "completed":
		return "☑"
	case "in_progress":
		return "◪"
	}
	return "☐"
}

func plural(n int, label string) string {
	if n == 1 {
		return "1 " + label
	}
	return strconv.Itoa(n) + " " + label + "s"
}

func pluralStatus(n int, label string) string {
	return strconv.Itoa(n) + " " + label
}
