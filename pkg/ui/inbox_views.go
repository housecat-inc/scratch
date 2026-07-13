package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/uikit"
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
	ChatOptions   []uikit.SelectOption
	Counts        InboxCounts
	Draft         *InboxDraftDetail
	Items         []InboxItem
	Selected      InboxSelection
	Task          *InboxTaskDetail
	Thread        *InboxThreadDetail
	View          string
	Workflow      *InboxWorkflowDetail
}

type InboxSelection struct {
	ID   int64
	Kind string
}

type InboxDraftDetail struct {
	Agent string
	Model string
	Title string
}

type InboxTaskDetail struct {
	Task db.Task
}

type InboxThreadDetail struct {
	Access      string
	Agent       string
	Archived    bool
	Description string
	ID          int64
	Kind        string
	Messages    []ChatMessageProps
	Starred     bool
	Streaming   bool
	Title       string
}

type InboxWorkflowDetail struct {
	Archived bool
	Awaiting bool
	ID       int64
	Items    []WorkflowItemProps
	Running  bool
	Starred  bool
	Status   string
	Title    string
}

type WorkflowItemProps struct {
	Answer   string
	Detail   string
	Duration string
	Failed   bool
	Form     *ChatFormProps
	Input    string
	Kind     string
	Running  bool
	Summary  string
	Title    string
}

type ReaderHeaderProps struct {
	BackHref    string
	Description string
	Labels      []string
	Title       string
}
