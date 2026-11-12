package ui

import (
	_ "embed"
	"strconv"

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

func ChatCSS() templ.Component {
	return templ.Raw(`<style>` + chatCSS + `</style>`)
}

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

