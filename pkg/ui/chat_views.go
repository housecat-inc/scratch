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

type ChatMessageProps struct {
	Author string
	Body   string
	Form   *ChatFormProps
	ID     int64
	Parts  []ChatPartProps
	Role   string
	Status string
}

type ChatPartProps struct {
	Kind string
	Plan []ChatPlanEntryProps
	Text string
	Tool *ChatToolCallProps
}

type ChatPlanEntryProps struct {
	Content string
	Status  string
}

type ChatToolCallProps struct {
	Detail string
	ID     string
	Status string
	Title  string
}

type ChatThreadProps struct {
	Agent    string
	ID       int64
	Messages []ChatMessageProps
	Title    string
}

func ChatCSS() templ.Component {
	return templ.Raw(`<style>` + chatCSS + `</style>`)
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func toolStatusIcon(status string) string {
	switch status {
	case "completed":
		return "✓"
	case "failed":
		return "✕"
	}
	return ""
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
