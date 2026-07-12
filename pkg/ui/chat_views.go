package ui

import (
	_ "embed"
	"strconv"

	"github.com/a-h/templ"
)

//go:embed static/chat.css
var chatCSS string

type ChatFormFieldProps struct {
	Label    string
	Name     string
	Options  []string
	Required bool
	Type     string
	Value    string
}

type ChatFormProps struct {
	Action        string
	ElicitationID string
	Fields        []ChatFormFieldProps
	Message       string
	MessageID     int64
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

type ChatThreadProps struct {
	Agent    string
	ID       int64
	Messages []ChatMessageProps
	Title    string
}

type FloatingChatProps struct {
	Access    string
	Agent     string
	ID        int64
	Messages  []ChatMessageProps
	Streaming bool
	Title     string
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

func planStatusIcon(status string) string {
	switch status {
	case "completed":
		return "☑"
	case "in_progress":
		return "◪"
	}
	return "☐"
}
