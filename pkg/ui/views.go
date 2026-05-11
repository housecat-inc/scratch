package ui

import (
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/git"
	"github.com/housecat-inc/scratch/pkg/repo"
)

type CodeSubnav struct {
	Active   string
	Comments int
	Repo     repo.Repo
}

type CommentForm struct {
	Anchor string
	Line   int
	Path   string
	Side   string
	Slug   string
}

type CommentItem struct {
	Anchor  string
	Comment db.Comment
	Slug    string
	View    string
}

type CommentListFile struct {
	Comments []db.Comment
	Path     string
}

type CommentListPage struct {
	Comments int
	Error    string
	Files    []CommentListFile
	Repo     repo.Repo
}

type CommentThread struct {
	Anchor   string
	Comments []db.Comment
	Slug     string
}

type CommitsPage struct {
	Commits  []git.Commit
	Comments int
	Error    string
	Repo     repo.Repo
}

type ContextResponse struct {
	Continuation *ExpandSpot
	Direction    string
	From         int
	HunkKey      string
	Lang         string
	Lines        []string
	Offset       int
}

type DiffLine struct {
	Anchor     string
	AnchorLine int
	Comments   []db.Comment
	Lang       string
	Line       git.Line
	Path       string
	Side       string
	Slug       string
}

type DiffPage struct {
	Comments int
	Error    string
	Files    []FileRow
	Repo     repo.Repo
}

type EditCommentForm struct {
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

type FileRow struct {
	Adds     int
	Binary   bool
	Comments int
	Dels     int
	Hunks    []*HunkBlock
	Path     string
	Slug     string
	Status   git.FileStatus
}

type FileTree struct {
	Entries []FileEntry
}

type FilesPage struct {
	Entries []FileEntry
	Error   string
	Root    string
}

type HunkBlock struct {
	Commit   git.Commit
	Hunk     *git.Hunk
	Lang     string
	Lines    []DiffLine
	PrevSpot *ExpandSpot
	Virtual  bool
}

type OverviewPage struct {
	Error string
	Home  string
	Repos []repo.Repo
}

type PickerView struct {
	Dir     string
	Entries []string
	Error   string
	HasUp   bool
	Parent  string
}

type SessionView struct {
	Dir         string
	ID          string
	LastMessage string
	Name        string
	Prompt      string
	StartedAt   time.Time
	URL         string
}

type SessionsView struct {
	AgentsBehind   int
	AgentsDir      string
	AgentsDirty    bool
	AgentsDiverged bool
	Authenticated  bool
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
	Sessions       []SessionView
}
