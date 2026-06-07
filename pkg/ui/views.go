package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
)

type CodeSubnavProps struct {
	Active   string
	Comments int
	Repo     repo.Repo
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
}

type PickerProps struct {
	Dir     string
	Entries []string
	Error   string
	HasUp   bool
	Parent  string
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
	AgentsBehind   int
	AgentsDir      string
	AgentsDirty    bool
	AgentsDiverged bool
	Authenticated  bool
	ClaudeVersion  string
	ConfigureError string
	Configured     bool
	InstallError   string
	Installed      bool
	LoginError     string
	LoginURL       string
	Nav            string
	Oob            bool
	SessionDir     string
	SessionError   string
	Sessions       []SessionProps
}
