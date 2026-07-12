package ui

import (
	_ "embed"
	"strconv"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/db"
)

//go:embed static/chat.css
var chatCSS string

type ChatIndexProps struct {
	Threads []db.Thread
}

type ChatMessageProps struct {
	Author string
	Body   string
	ID     int64
	Role   string
	Status string
	Tools  []string
}

type ChatThreadProps struct {
	ID       int64
	Messages []ChatMessageProps
	Title    string
}

func ChatCSS() templ.Component {
	return templ.Raw(`<style>` + chatCSS + `</style>`)
}

func chatPath(id int64) string { return "/chat/" + itoa64(id) }

func itoa64(n int64) string { return strconv.FormatInt(n, 10) }

func threadTitle(t db.Thread) string {
	if t.Title == "" {
		return "Untitled"
	}
	return t.Title
}
