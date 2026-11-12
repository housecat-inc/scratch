// Package icons pins the app's semantic icon choices in one place, so a glyph can
// be swapped everywhere it appears by editing a single line here.
//
// The lucide data it draws from lives in internal/lucide, moved out of templui's
// ui/components/icon so `templui add` cannot overwrite it and so nothing outside
// this package can import it.
package icons

import (
	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/icons/internal/lucide"
)

// Name is a semantic icon, not a lucide one: the constant names what the glyph
// means to the product and its value pins which lucide glyph draws it, so a
// swap is one edit here rather than a search across every call site. Components
// that take an icon take a Name, which keeps lucide's vocabulary out of call
// sites and makes the set of icons in use enumerable.
type Name string

const (
	// Action is an automation from the catalog, run on demand.
	Action     Name = "action"
	ActionOff  Name = "action-off"
	Activity   Name = "activity"
	Add        Name = "add"
	AddContact Name = "add-contact"
	// Agent is the in-app agent itself, as opposed to Step, one thing it ran.
	Agent Name = "agent"
	// Agreement is a deal being struck, drawn as a handshake where a list
	// needs to tell deals from the companies they belong to.
	Agreement  Name = "agreement"
	Alert      Name = "alert"
	AllMail    Name = "all-mail"
	APIKey     Name = "api-key"
	Archive    Name = "archive"
	Attachment Name = "attachment"
	Blocked    Name = "blocked"
	Bold       Name = "bold"
	BulletList Name = "bullet-list"
	// Calendar is a calendar as a whole -- a link to one, a date further out -- as
	// opposed to Date, a single date field.
	Calendar        Name = "calendar"
	Chat            Name = "chat"
	ChatNew         Name = "chat-new"
	Checkbox        Name = "checkbox"
	Checked         Name = "checked"
	ClearFormatting Name = "clear-formatting"
	Close           Name = "close"
	CodeEditor      Name = "code-editor"
	// CollapseLeft and CollapseRight close a panel by sending it back toward
	// the edge it came from. The direction is part of the name on purpose,
	// because the glyph points that way and the two edges are not
	// interchangeable.
	CollapseLeft  Name = "collapse-left"
	CollapseRight Name = "collapse-right"
	// Command is the key a keyboard shortcut is held with, drawn by kbd.Kbd.
	Command Name = "command"
	// Company is an organisation record in the CRM, as opposed to Deal.
	Company Name = "company"
	// Complete marks something finished, as opposed to Done, the bare tick
	// that confirms an action.
	Complete Name = "complete"
	// Compose is the square-with-a-pencil glyph for starting something new --
	// a mail draft, a chat -- as opposed to Edit, which changes something that
	// already exists.
	Compose       Name = "compose"
	Connect       Name = "connect"
	ContactCard   Name = "contact-card"
	Conversations Name = "conversations"
	Copy          Name = "copy"
	// Date is a date field or picker.
	Date Name = "date"
	// Deal is a CRM record for an organisation -- a deal, a company, an account
	// -- as opposed to AddContact, which is a person.
	Deal Name = "deal"
	// Delegate hands a task to someone else. It shares AddContact's glyph but
	// is a different act, so a glyph of its own later is one edit here.
	Delegate   Name = "delegate"
	Delete     Name = "delete"
	Disconnect Name = "disconnect"
	Document   Name = "document"
	Done       Name = "done"
	// Download saves a file to the device.
	Download   Name = "download"
	Draft      Name = "draft"
	DragHandle Name = "drag-handle"
	Edit       Name = "edit"
	// ExitFullScreen shrinks a full-screen panel back to its usual size, the
	// inverse of FullScreen.
	ExitFullScreen Name = "exit-full-screen"
	ExpandAll      Name = "expand-all"
	// ExpandDown and ExpandRight carry their direction in the name on purpose:
	// "expand" alone says nothing about which way the chevron points, and the
	// controls differ. ExpandDown opens below itself -- a select, a list that
	// loads more beneath it -- as opposed to OpenMenu, whose chevrons point both
	// ways. ExpandRight opens or leads to its right -- a submenu, a palette
	// drill-in, a breadcrumb's next level -- or is a collapsed disclosure that
	// rotates down as it opens.
	ExpandDown  Name = "expand-down"
	ExpandRight Name = "expand-right"
	// ExternalLink marks a link that leaves the app, such as a step that opens
	// another site to finish sign-in.
	ExternalLink Name = "external-link"
	Failed       Name = "failed"
	Feedback     Name = "feedback"
	// Files is the user's file browser.
	Files Name = "files"
	// Filter narrows a list to the items matching a choice.
	Filter Name = "filter"
	// ForwardMessage is forwarding a message on. The arrows that move through a
	// history are GoBack and GoForward, named so that neither can be reached for
	// by typing the shorter word.
	ForwardMessage Name = "forward-message"
	// FullScreen grows a panel to fill the window, as the composer's does.
	FullScreen Name = "full-screen"
	GitHub     Name = "github"
	// GoBack and GoForward step through a history -- a thread you opened from a
	// list, a wizard. They are the plain arrows, and say Go because forwarding a
	// message is a different thing with a glyph of its own.
	GoBack    Name = "go-back"
	GoForward Name = "go-forward"
	// Heading is the one heading level the editor offers, so the name says
	// heading rather than which level: a strip with a single heading tool wants
	// "Heading" on its tooltip whatever the tool inserts.
	Heading Name = "heading"
	Help    Name = "help"
	Hide    Name = "hide"
	// Home is the sidebar's link back to the home page.
	Home  Name = "home"
	Inbox Name = "inbox"
	// Info opens details about the thing it sits on, as opposed to Help, which
	// explains how to use it.
	Info   Name = "info"
	Italic Name = "italic"
	Jobs   Name = "jobs"
	Label  Name = "label"
	Labels Name = "labels"
	// Later is the mailbox of everything not sorted anywhere else.
	Later      Name = "later"
	Link       Name = "link"
	Loading    Name = "loading"
	LogIn      Name = "log-in"
	LogOut     Name = "log-out"
	Mail       Name = "mail"
	MailSearch Name = "mail-search"
	// MarkImportant and MarkNotImportant set a conversation's importance.
	// MarkImportant draws a bookmark and MarkNotImportant the struck-out one, so
	// each glyph shows the state the conversation is put in.
	MarkImportant    Name = "mark-important"
	MarkNotImportant Name = "mark-not-important"
	// MarkRead and MarkUnread set a conversation's read state. MarkUnread draws
	// Mail's closed envelope and MarkRead the open one, so each glyph shows the
	// state the conversation is put in.
	MarkRead     Name = "mark-read"
	MarkUnread   Name = "mark-unread"
	MeetingNotes Name = "meeting-notes"
	// Member is a user record in a list, as opposed to Person, who is signed in.
	Member  Name = "member"
	Menu    Name = "menu"
	Message Name = "message"
	// Minimize folds a window down to its title bar, drawn as the bar itself.
	Minimize Name = "minimize"
	More     Name = "more"
	// Move files a conversation under a label, out of the inbox.
	Move         Name = "move"
	NewerPage    Name = "newer-page"
	NewEvent     Name = "new-event"
	Note         Name = "note"
	Notebook     Name = "notebook"
	NotSpam      Name = "not-spam"
	NumberedList Name = "numbered-list"
	OlderPage    Name = "older-page"
	// OpenMenu marks a trigger that opens a menu against it, as opposed to
	// ExpandAll, which expands every collapsed row in a list.
	OpenMenu Name = "open-menu"
	Packages Name = "packages"
	Pause    Name = "pause"
	Paused   Name = "paused"
	// Pending is the empty circle a task shows until it is Complete.
	Pending Name = "pending"
	// Person is whoever is signed in, as opposed to AddContact, which is a
	// person being written down in the CRM.
	Person  Name = "person"
	Phone   Name = "phone"
	PopOut  Name = "pop-out"
	Power   Name = "power"
	Profile Name = "profile"
	Project Name = "project"
	// Queue sends a message that waits behind the running turn, so it is
	// Forward's arrow stopped at a line rather than a glyph of its own.
	Queue      Name = "queue"
	Quote      Name = "quote"
	Redo       Name = "redo"
	Refresh    Name = "refresh"
	Remind     Name = "remind"
	Remove     Name = "remove"
	Reply      Name = "reply"
	ReplyAll   Name = "reply-all"
	Research   Name = "research"
	ResetOrder Name = "reset-order"
	// Retry re-runs something that already failed -- a job, a workflow step --
	// as opposed to Run, which starts one for the first time.
	Retry   Name = "retry"
	Run     Name = "run"
	Running Name = "running"
	Save    Name = "save"
	// Schedule sets something for a later date -- an event, a reminder a week
	// out -- as opposed to Remind, which nudges about a message.
	Schedule Name = "schedule"
	// Screenshot captures part of the page as an image.
	Screenshot Name = "screenshot"
	Search     Name = "search"
	Security   Name = "security"
	// Send commits a message a person has composed -- a chat submit, a reply.
	// It draws the same arrow as GoForward for now, but it is a different act
	// from stepping through a history, so it gets its own name: giving it a
	// glyph of its own later is then one edit here.
	Send Name = "send"
	// Sent is the mailbox, drawn as a paper plane; Send, the act, is an arrow.
	Sent     Name = "sent"
	Settings Name = "settings"
	Show     Name = "show"
	Skills   Name = "skills"
	Slack    Name = "slack"
	Sleep    Name = "sleep"
	// Source is the raw markup or data behind something the user sees, as
	// opposed to CodeEditor, the place code is written.
	Source      Name = "source"
	Spam        Name = "spam"
	Spreadsheet Name = "spreadsheet"
	// Starred is the mailbox of starred mail, drawn outlined; the star toggle
	// itself is ui/components/star.
	Starred Name = "starred"
	// Step is one thing an agent ran on the way to an answer, drawn as a
	// terminal prompt, as opposed to Tool, the wrench for a single tool.
	Step          Name = "step"
	Stop          Name = "stop"
	Storage       Name = "storage"
	Strikethrough Name = "strikethrough"
	// Suggest marks something the app worked out on its own, rather than
	// something the user asked for by name.
	Suggest Name = "suggest"
	// Task is one task, or the way to the task list, as opposed to Tasks, the
	// list itself.
	Task           Name = "task"
	Tasks          Name = "tasks"
	TemplateCreate Name = "template-create"
	// TemplateReply answers a message from a template, as opposed to
	// TemplateCreate, which starts a new template.
	TemplateReply Name = "template-reply"
	Templates     Name = "templates"
	ThemeSystem   Name = "theme-system"
	Thinking      Name = "thinking"
	Timer         Name = "timer"
	Tool          Name = "tool"
	// ToolPalette opens the palette of tools for a row, as opposed to Tool, the
	// wrench that stands for a single tool.
	ToolPalette Name = "tool-palette"
	// Trash is the plain bin the auth service's delete buttons draw; the app
	// deletes with Delete, lucide's lidded trash-2, and the two stay distinct
	// so each service keeps the glyph it has.
	Trash Name = "trash"
	// Unarchive brings back anything archived -- mail, a chat, a template --
	// drawn as Archive's box with an arrow coming out of it. Archive and
	// Unarchive are the one pair every archive control uses.
	Unarchive Name = "unarchive"
	Underline Name = "underline"
	Unstar    Name = "unstar"
	Upgrade   Name = "upgrade"
	// Users is everyone with an account, as opposed to Person, who is signed in.
	Users Name = "users"
	VM    Name = "vm"
	// Waiting marks a control held until something in flight finishes, and
	// is drawn spinning by the caller.
	Waiting   Name = "waiting"
	Webhook   Name = "webhook"
	Workflows Name = "workflows"
	// WriteAI drafts text with the assistant.
	WriteAI Name = "write-ai"
)

// glyphs is the whole mapping. A glyph swap is one edit here and every call
// site follows, which is the point of naming icons by meaning: nothing outside
// this file knows or cares that "more" is drawn with ellipsis-vertical.
var glyphs = map[Name]string{
	Action:           "zap",
	ActionOff:        "zap-off",
	Activity:         "activity",
	Add:              "plus",
	AddContact:       "user-round-plus",
	Agent:            "bot",
	Agreement:        "handshake",
	Alert:            "circle-alert",
	AllMail:          "mails",
	APIKey:           "key",
	Archive:          "archive",
	Attachment:       "paperclip",
	Blocked:          "ban",
	Bold:             "bold",
	BulletList:       "list",
	Calendar:         "calendar-days",
	Chat:             "message-square-text",
	ChatNew:          "message-square-plus",
	Checkbox:         "square-check",
	Checked:          "square-check-big",
	ClearFormatting:  "remove-formatting",
	Close:            "x",
	CodeEditor:       "code-xml",
	CollapseLeft:     "chevron-left",
	CollapseRight:    "chevron-right",
	Command:          "command",
	Company:          "building",
	Complete:         "circle-check",
	Compose:          "square-pen",
	Connect:          "plug",
	ContactCard:      "contact",
	Conversations:    "messages-square",
	Copy:             "copy",
	Date:             "calendar",
	Deal:             "building-2",
	Delegate:         "user-round-plus",
	Delete:           "trash-2",
	Disconnect:       "unplug",
	Document:         "file-text",
	Done:             "check",
	Download:         "download",
	Draft:            "pen",
	DragHandle:       "grip-vertical",
	Edit:             "pencil",
	ExitFullScreen:   "minimize",
	ExpandAll:        "chevrons-up-down",
	ExpandDown:       "chevron-down",
	ExpandRight:      "chevron-right",
	ExternalLink:     "external-link",
	Failed:           "circle-x",
	Feedback:         "megaphone",
	Files:            "folder",
	Filter:           "list-filter",
	ForwardMessage:   "forward",
	FullScreen:       "maximize",
	GitHub:           "github",
	GoBack:           "arrow-left",
	GoForward:        "arrow-right",
	Heading:          "heading-2",
	Help:             "circle-question-mark",
	Hide:             "eye-off",
	Home:             "house",
	Inbox:            "inbox",
	Info:             "info",
	Italic:           "italic",
	Jobs:             "briefcase",
	Label:            "tag",
	Labels:           "tags",
	Later:            "circle-ellipsis",
	Link:             "link",
	Loading:          "loader",
	LogIn:            "log-in",
	LogOut:           "log-out",
	Mail:             "mail",
	MailSearch:       "mail-search",
	MarkImportant:    "bookmark",
	MarkNotImportant: "bookmark-x",
	MarkRead:         "mail-open",
	MarkUnread:       "mail",
	MeetingNotes:     "notebook-text",
	Member:           "user",
	Menu:             "menu",
	Message:          "message-square",
	Minimize:         "rectangle-horizontal",
	More:             "ellipsis-vertical",
	Move:             "folder-input",
	NewerPage:        "chevron-left",
	NewEvent:         "calendar-plus",
	Note:             "sticky-note",
	Notebook:         "notebook",
	NotSpam:          "shield-check",
	NumberedList:     "list-ordered",
	OlderPage:        "chevron-right",
	OpenMenu:         "chevrons-up-down",
	Packages:         "boxes",
	Pause:            "pause",
	Paused:           "circle-pause",
	Pending:          "circle",
	Person:           "user-round",
	Phone:            "phone",
	PopOut:           "square-arrow-out-up-right",
	Power:            "power",
	Profile:          "circle-user",
	Project:          "folder-open",
	Queue:            "arrow-right-to-line",
	Quote:            "text-quote",
	Redo:             "rotate-cw",
	Refresh:          "refresh-cw",
	Remind:           "clock",
	Remove:           "minus",
	Reply:            "reply",
	ReplyAll:         "reply-all",
	Research:         "flask-conical",
	ResetOrder:       "list-restart",
	Retry:            "rotate-ccw",
	Run:              "play",
	Running:          "circle-play",
	Save:             "save",
	Schedule:         "calendar-clock",
	Screenshot:       "camera",
	Search:           "search",
	Security:         "shield",
	Send:             "arrow-right",
	Sent:             "send",
	Settings:         "settings",
	Show:             "eye",
	Skills:           "sparkles",
	Slack:            "slack",
	Sleep:            "moon",
	Source:           "code",
	Spam:             "shield-alert",
	Spreadsheet:      "sheet",
	Starred:          "star",
	Step:             "terminal",
	Stop:             "square",
	Storage:          "hard-drive",
	Strikethrough:    "strikethrough",
	Suggest:          "sparkles",
	Task:             "square-check-big",
	Tasks:            "list-checks",
	TemplateCreate:   "file-plus-corner",
	TemplateReply:    "file-output",
	Templates:        "file-braces-corner",
	ThemeSystem:      "eclipse",
	Thinking:         "brain",
	Timer:            "timer",
	Tool:             "wrench",
	ToolPalette:      "layers",
	Trash:            "trash",
	Unarchive:        "archive-restore",
	Underline:        "underline",
	Unstar:           "star-off",
	Upgrade:          "circle-arrow-up",
	Users:            "users",
	VM:               "server",
	Waiting:          "loader-circle",
	Webhook:          "webhook",
	Workflows:        "workflow",
	WriteAI:          "wand-sparkles",
}

// All is every semantic icon, in the order a picker should show them. Anything
// enumerating the library -- a Storybook control, a future icon browser --
// reads this rather than keeping its own copy that quietly falls behind.
var All = []Name{
	Action,
	ActionOff,
	Activity,
	Add,
	AddContact,
	Agent,
	Agreement,
	Alert,
	AllMail,
	APIKey,
	Archive,
	Attachment,
	Blocked,
	Bold,
	BulletList,
	Calendar,
	Chat,
	ChatNew,
	Checkbox,
	Checked,
	ClearFormatting,
	Close,
	CodeEditor,
	CollapseLeft,
	CollapseRight,
	Command,
	Company,
	Complete,
	Compose,
	Connect,
	ContactCard,
	Conversations,
	Copy,
	Date,
	Deal,
	Delegate,
	Delete,
	Disconnect,
	Document,
	Done,
	Download,
	Draft,
	DragHandle,
	Edit,
	ExitFullScreen,
	ExpandAll,
	ExpandDown,
	ExpandRight,
	ExternalLink,
	Failed,
	Feedback,
	Files,
	Filter,
	ForwardMessage,
	FullScreen,
	GitHub,
	GoBack,
	GoForward,
	Heading,
	Help,
	Hide,
	Home,
	Inbox,
	Info,
	Italic,
	Jobs,
	Label,
	Labels,
	Later,
	Link,
	Loading,
	LogIn,
	LogOut,
	Mail,
	MailSearch,
	MarkImportant,
	MarkNotImportant,
	MarkRead,
	MarkUnread,
	MeetingNotes,
	Member,
	Menu,
	Message,
	Minimize,
	More,
	Move,
	NewerPage,
	NewEvent,
	Note,
	Notebook,
	NotSpam,
	NumberedList,
	OlderPage,
	OpenMenu,
	Packages,
	Pause,
	Paused,
	Pending,
	Person,
	Phone,
	PopOut,
	Power,
	Profile,
	Project,
	Queue,
	Quote,
	Redo,
	Refresh,
	Remind,
	Remove,
	Reply,
	ReplyAll,
	Research,
	ResetOrder,
	Retry,
	Run,
	Running,
	Save,
	Schedule,
	Screenshot,
	Search,
	Security,
	Send,
	Sent,
	Settings,
	Show,
	Skills,
	Slack,
	Sleep,
	Source,
	Spam,
	Spreadsheet,
	Starred,
	Step,
	Stop,
	Storage,
	Strikethrough,
	Suggest,
	Task,
	Tasks,
	TemplateCreate,
	TemplateReply,
	Templates,
	ThemeSystem,
	Thinking,
	Timer,
	Tool,
	ToolPalette,
	Trash,
	Unarchive,
	Underline,
	Unstar,
	Upgrade,
	Users,
	VM,
	Waiting,
	Webhook,
	Workflows,
	WriteAI,
}

// Names is All as plain strings, for control options.
func Names() []string {
	out := make([]string, len(All))
	for i, n := range All {
		out[i] = string(n)
	}
	return out
}

// Props is what a call site tunes a glyph with -- size, stroke, class. It
// aliases the internal lucide package's Props, which nothing outside ui/icons
// can import.
type Props = lucide.Props

// Render draws the glyph a Name stands for. It lives here rather than in the
// icon package so a component can depend on the semantic set alone. An unknown
// name draws nothing rather than erroring: it can only come from a control or a
// query string, and a blank button is a clearer signal than a broken page.
func Render(name Name, props ...lucide.Props) templ.Component {
	glyph, ok := glyphs[name]
	if !ok {
		return templ.NopComponent
	}
	return lucide.Icon(glyph)(props...)
}

// Glyph is Render bound to one name, for fields that take a component
// constructor rather than a Name.
func Glyph(name Name) func(...lucide.Props) templ.Component {
	return func(props ...lucide.Props) templ.Component { return Render(name, props...) }
}
