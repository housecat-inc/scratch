package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/workflow"
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
	Page           *db.Page
	Pages          []db.Page
	ArchiveFilter  string
	ChatOptions    []uikit.SelectOption
	Counts         InboxCounts
	Draft          *InboxDraftDetail
	Items          []InboxItem
	Schedule       *InboxScheduleDetail
	Selected       InboxSelection
	Task           *InboxTaskDetail
	Thread         *InboxThreadDetail
	View           string
	WorkflowWizard bool
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
	RunColumns  []string
	Runs        []InboxScheduleRun
	Starred     bool
	Steps       []workflow.StepDefinition
	Streaming   bool
	Title       string
}

type ReaderHeaderProps struct {
	BackHref    string
	Description string
	Labels      []string
	Title       string
}

type InboxScheduleDetail struct {
	Created       string
	Updated       string
	Cron          string
	Timezone      string
	LastRun       string
	Last24h       int64
	SuccessRate   string
	ScheduleLabel string
	AuditURL      string
	Description   string
	Archived      bool
	ID            int64
	NextRun       string
	RunColumns    []string
	Runs          []InboxScheduleRun
	Starred       bool
	Status        string
	Steps         []workflow.StepDefinition
	Title         string
	TotalRuns     int64
	Triggers      string
}

type InboxScheduleRun struct {
	AuditURL string
	At       string
	Cells    []string
	ID       string
	Result   string
	Status   string
}
