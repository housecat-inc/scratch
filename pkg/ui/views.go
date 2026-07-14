package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/uikit"
)

type CodeSubnavProps struct {
	Active   string
	Comments int
	Repo     repo.Repo
	Shell    ToolShellProps
}

type CommentFormProps struct {
	Anchor string
	Line   int
	Path   string
	Side   string
	Slug   string
}

type CommentItemProps struct {
	Anchor  string
	Comment db.Comment
	Slug    string
	View    string
}

type CommentListFileProps struct {
	Comments []db.Comment
	Path     string
	Slug     string
}

type CommentListProps struct {
	Comments int
	Error    string
	Files    []CommentListFileProps
	Repo     repo.Repo
	Shell    ToolShellProps
}

type CommentThreadProps struct {
	Anchor   string
	Comments []db.Comment
	Slug     string
}

type CommitsProps struct {
	Commits  []git.Commit
	Comments int
	Error    string
	Repo     repo.Repo
	Shell    ToolShellProps
}

type ContextProps struct {
	Continuation *ExpandSpot
	Direction    string
	From         int
	HunkKey      string
	Lang         string
	Lines        []string
	Offset       int
}

type DiffLineProps struct {
	Anchor     string
	AnchorLine int
	Comments   []db.Comment
	Lang       string
	Line       git.Line
	Path       string
	Side       string
	Slug       string
}

type DiffProps struct {
	Commit   git.Commit
	Comments int
	Error    string
	Files    []FileProps
	Repo     repo.Repo
	Shell    ToolShellProps
}

type EditCommentFormProps struct {
	Anchor  string
	Comment db.Comment
	Slug    string
	View    string
}

type ExpandSpot struct {
	Bound     int
	Direction string
	From      int
	FullFrom  int
	FullTo    int
	HunkKey   string
	Offset    int
	Path      string
	Slug      string
	To        int
}

type FileEntry struct {
	Dir  bool
	Name string
	Path string
}

type FileProps struct {
	Adds     int
	Binary   bool
	Comments int
	Dels     int
	Hunks    []*HunkProps
	Path     string
	Slug     string
	Status   git.FileStatus
}

type FileTreeProps struct {
	Entries []FileEntry
}

type FilesProps struct {
	Entries []FileEntry
	Error   string
	Root    string
	Shell   ToolShellProps
}

type HunkProps struct {
	Commit   git.Commit
	Hunk     *git.Hunk
	Lang     string
	Lines    []DiffLineProps
	PrevSpot *ExpandSpot
	Virtual  bool
}

type OverviewProps struct {
	Error string
	Home  string
	Repos []repo.Repo
	Shell ToolShellProps
}

type PickerProps struct {
	Dir     string
	Entries []string
	Error   string
	HasUp   bool
	Parent  string
}

type SQLColumn struct {
	Name string
	Type string
}

type SQLProps struct {
	DBFiles []string
	Error   string
	Path    string
	Query   string
	Result  *SQLResult
	Saved   []db.SQLQuery
	Shell   ToolShellProps
	Tables  []SQLTable
}

type SQLCell struct {
	Null  bool
	Value string
}

type SQLResult struct {
	Columns   []string
	Elapsed   string
	Error     string
	Rows      [][]SQLCell
	Truncated bool
}

type SQLTable struct {
	Columns []SQLColumn
	Name    string
}

type SessionProps struct {
	Dir         string
	ID          string
	LastMessage string
	Name        string
	Prompt      string
	StartedAt   time.Time
	URL         string
}

type SessionsProps struct {
	AgentsBehind       int
	AgentsDir          string
	AgentsDirty        bool
	AgentsDiverged     bool
	Authenticated      bool
	ClaudeVersion      string
	CodexAuthenticated bool
	CodexInstalled     bool
	CodexLoginCode     string
	CodexLoginError    string
	CodexLoginURL      string
	CodexVersion       string
	ConfigureError     string
	Configured         bool
	InstallError       string
	Installed          bool
	LoginError         string
	LoginURL           string
	Nav                string
	Oob                bool
	SessionDir         string
	SessionError       string
	Sessions           []SessionProps
	Shell              ToolShellProps
}

type ToolShellProps struct {
	ChatOptions []uikit.SelectOption
	Counts      InboxCounts
}
