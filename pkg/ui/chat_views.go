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

type ChatIndexProps struct {
	Agents  []string
	Threads []ChatThreadItemProps
}

type ChatThreadItemProps struct {
	Agent string
	ID    int64
	Title string
	When  string
}

type ChatMessageProps struct {
	Author string
	Body   string
	Form   *ChatFormProps
	ID     int64
	Role   string
	Status string
	Tools  []string
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

func chatPath(id int64) string { return "/chat/" + itoa64(id) }

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }
