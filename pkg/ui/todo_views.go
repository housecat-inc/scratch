package ui

import (
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/uikit"
)

type TodoChatItem struct {
	Description string
	ID          int64
	Title       string
	When        string
}

type TodoProps struct {
	ActiveCount int
	ChatCount   int
	ChatItems   []TodoChatItem
	ChatLabel   string
	ChatOptions []uikit.SelectOption
	Detail      *TodoTaskDetail
	Title       string
	Tasks       []db.Task
	TaskCount   int
	View        string
}

type TodoTaskDetail struct {
	Task db.Task
}
