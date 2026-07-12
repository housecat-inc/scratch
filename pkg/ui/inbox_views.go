package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
)

type InboxCounts struct {
	Chats     int
	Inbox     int
	Starred   int
	Tasks     int
	Workflows int
}

type InboxItem struct {
	Archived  bool
	Done      bool
	From      string
	Href      string
	ID        int64
	Kind      string
	Snippet   string
	Starred   bool
	Title     string
	UpdatedAt time.Time
	When      string
	Workflow  bool
}

type InboxProps struct {
	ArchiveFilter string
	Agents        []string
	Counts        InboxCounts
	Items         []InboxItem
	Selected      InboxSelection
	Task          *InboxTaskDetail
	Thread        *InboxThreadDetail
	View          string
}

type InboxSelection struct {
	ID   int64
	Kind string
}

type InboxTaskDetail struct {
	Notes    []db.TaskNote
	Subitems []db.TaskSubitem
	Task     db.Task
}

type InboxThreadDetail struct {
	Agent    string
	Archived bool
	ID       int64
	Kind     string
	Messages []ChatMessageProps
	Starred  bool
	Title    string
}

type SelectOption struct {
	Label string
	Value string
}
