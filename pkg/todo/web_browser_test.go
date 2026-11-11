package todo

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/testkit"
)

type webCase struct {
	act    func(h *webHarness)
	assert func(h *webHarness)
	name   string
	path   string
	seed   func(h *webHarness)
}

type webHarness struct {
	*testkit.Harness
	Chat  *chat.Service
	Tasks *Service
}

const railPrompt = "#mail-chat-rail-content [data-prompt-field]"

type webTaskOption func(*Service, db.Task) (db.Task, error)

func TestWebBrowser(t *testing.T) {
	t.Run("chat panel", func(t *testing.T) {
		tests := []webCase{
			{
				act: func(h *webHarness) {
					h.Click("[data-chat-rail-toggle]")
					h.eventuallyPresent(railPrompt)
					h.Type(railPrompt, "hello todo")
					h.Press(railPrompt, "Enter")
				},
				assert: func(h *webHarness) {
					h.chatThreadCount(1)
					h.chatThreadLabel(1, chatLabelTodo)
					h.ElementTextContains("#mail-chat-rail-content [data-chat-entries]", "You said: hello todo")
				},
				name: "chat toggle starts a todo chat in the panel",
				path: "/",
			},
			{
				act: func(h *webHarness) {
					h.Click("[data-chat-rail-toggle]")
					h.eventuallyPresent(railPrompt)
					h.Click(`[data-slot="nav-item"][href="/tasks"]`)
				},
				assert: func(h *webHarness) {
					h.pathEventuallyEquals("/tasks")
					h.eventuallyPresent(railPrompt)
				},
				name: "survives navigation",
				path: "/",
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				runWebCase(t, tt)
			})
		}
	})

	t.Run("inbox", func(t *testing.T) {
		tests := []webCase{
			{
				act: func(h *webHarness) {
					h.Click(`[aria-label="New task"]`)
				},
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-reader-title", "New task")
					h.ElementTextContains(".mail-reader-labels", "Active")
					h.ElementAbsent("section.mail-card")
				},
				name: "creates a task",
				path: "/",
			},
			{
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-mainbar", "Inbox")
					h.ElementTextContains(`[data-id="1"]`, "active one")
					h.ElementVisible(`[data-id="1"] form[action="/tasks/1/done"] button`)
					h.ElementVisible(`[data-id="1"] button[aria-label="Archive"]`)
					h.ElementAbsent(".mail-reader-head")
					h.ElementAbsent(`[data-id="2"]`)
					h.ElementAbsent(`[data-id="3"]`)
				},
				name: "lists only active tasks",
				path: "/",
				seed: func(h *webHarness) {
					h.seedTask("active one")
					h.seedTask("done one", webTaskCompleted(true))
					h.seedTask("archived one", webTaskArchived(true))
				},
			},
			{
				act: func(h *webHarness) {
					h.Click(".gm-row-link")
				},
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-reader-title", "open task")
					h.ElementTextContains(".mail-reader-labels", "Active")
					h.ElementAbsent("section.mail-card")
				},
				name: "opens a task page from a row",
				path: "/",
				seed: func(h *webHarness) {
					h.seedTask("open task")
				},
			},
			{
				act: func(h *webHarness) {
					h.hover(`[data-id="1"]`)
					h.Click(`[data-id="1"] button[aria-label="Archive"]`)
					h.Load("/tasks")
					h.hover(`[data-id="1"]`)
					h.Click(`[data-id="1"] button[aria-label="Unarchive"]`)
				},
				assert: func(h *webHarness) {
					h.taskEventually(1, func(task db.Task) bool { return !task.Archived })
					h.ElementVisible(`[data-id="1"] button[aria-label="Archive"]`)
				},
				name: "archives and unarchives from list actions",
				path: "/",
				seed: func(h *webHarness) {
					h.seedTask("archive me")
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				runWebCase(t, tt)
			})
		}
	})

	t.Run("tasks list", func(t *testing.T) {
		tests := []webCase{
			{
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-mainbar", "Tasks")
					h.ElementTextContains(`[data-id="1"]`, "active one")
					h.ElementTextContains(`[data-id="2"]`, "done one")
					h.ElementTextContains(`[data-id="3"]`, "archived one")
					h.ElementVisible(`[data-id="3"] button[aria-label="Unarchive"]`)
					h.ElementVisible(`[data-id="3"] button[aria-label="Delete"]`)
					h.ElementAbsent(".mail-reader-head")
				},
				name: "lists all tasks with archived actions",
				path: "/tasks",
				seed: func(h *webHarness) {
					h.seedTask("active one")
					h.seedTask("done one", webTaskCompleted(true))
					h.seedTask("archived one", webTaskArchived(true))
				},
			},
			{
				act: func(h *webHarness) {
					h.hover(`[data-id="1"]`)
					h.Click(`[data-id="1"] button[aria-label="Delete"]`)
					h.taskEventually(1, func(db.Task) bool { return true })
					h.ElementVisible(`[data-id="1"] [data-trash-submit]`)
					h.Click(`[data-id="1"] [data-trash-submit]`)
				},
				assert: func(h *webHarness) {
					h.taskEventuallyDeleted(1)
					h.ElementAbsent(`[data-id="1"]`)
				},
				name: "deletes an archived task after confirmation",
				path: "/tasks",
				seed: func(h *webHarness) {
					h.seedTask("remove me", webTaskArchived(true))
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				runWebCase(t, tt)
			})
		}
	})

	t.Run("task page", func(t *testing.T) {
		tests := []webCase{
			{
				act: func(h *webHarness) {
					h.Click(`.mail-reader-head form[action="/tasks/1/done"] button`)
				},
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-reader-labels", "Done")
					h.taskEventually(1, func(task db.Task) bool { return task.Completed })
				},
				name: "toggles completion",
				path: "/tasks/1",
				seed: func(h *webHarness) {
					h.seedTask("draft")
				},
			},
			{
				act: func(h *webHarness) {
					h.Type(`[name="description"]`, "Call customer")
					h.Click(".mail-save-btn")
				},
				assert: func(h *webHarness) {
					h.taskEventually(1, func(task db.Task) bool { return task.Description == "Call customer" })
					h.ElementTextContains(".mail-reader-title", "draft")
				},
				name: "saves the description and keeps the title",
				path: "/tasks/1",
				seed: func(h *webHarness) {
					h.seedTask("draft")
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				runWebCase(t, tt)
			})
		}
	})

	t.Run("chats list", func(t *testing.T) {
		tests := []webCase{
			{
				assert: func(h *webHarness) {
					h.ElementTextContains(".mail-mainbar", "Chats")
					h.ElementTextContains(`[data-kind="chat"][data-id="1"]`, "old chat")
					h.ElementAbsent(`[data-kind="chat"][data-id="2"]`)
				},
				name: "lists todo chats without scratch chats",
				path: "/chats",
				seed: func(h *webHarness) {
					h.seedChat("old chat", chatLabelTodo)
					h.seedChat("scratch chat", "")
				},
			},
			{
				act: func(h *webHarness) {
					h.Click(`[data-kind="chat"][data-id="1"] .gm-row-link`)
				},
				assert: func(h *webHarness) {
					h.pathEventuallyEquals("/chats")
					h.eventuallyPresent(railPrompt)
					h.ElementTextContains("#mail-chat-rail-content", "old chat")
				},
				name: "opens a todo chat row in the chat panel",
				path: "/chats",
				seed: func(h *webHarness) {
					h.seedChat("old chat", chatLabelTodo)
				},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				runWebCase(t, tt)
			})
		}
	})
}

func newWebHarness(t *testing.T) *webHarness {
	t.Helper()
	kit := testkit.New(t)

	store, err := db.New(":memory:")
	kit.R.NoError(err)
	t.Cleanup(func() { store.Close() })

	tasks := NewService(store)
	chatSvc := chat.NewService(store, chat.EchoAgent{Delay: 10 * time.Millisecond}, slog.Default())
	t.Cleanup(chatSvc.Close)
	mux := http.NewServeMux()
	mux.Handle("/chat/", chat.NewServer(chatSvc, slog.Default()).Handler())
	mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
	codex := chat.NewCodexServer(chatSvc).WithLabel(ChatLabel)
	mux.Handle("/codex", codex.Handler())
	mux.Handle("/codex/", codex.Handler())
	mux.Handle("/", codex.Middleware(NewWebServerWithChat(tasks, chatSvc, slog.Default()).Handler()))

	return &webHarness{
		Chat:    chatSvc,
		Harness: testkit.NewHarnessWithT(t, kit, mux),
		Tasks:   tasks,
	}
}

func runWebCase(t *testing.T, tc webCase) {
	t.Helper()
	h := newWebHarness(t)
	if tc.seed != nil {
		tc.seed(h)
	}
	h.Load(tc.path)
	if tc.act != nil {
		tc.act(h)
	}
	tc.assert(h)
	for _, msg := range h.Console.Errors() {
		t.Errorf("unexpected console error: %s", msg)
	}
}

func webTaskArchived(archived bool) webTaskOption {
	return func(tasks *Service, task db.Task) (db.Task, error) {
		return tasks.SetArchived(task.ID, archived)
	}
}

func webTaskCompleted(completed bool) webTaskOption {
	return func(tasks *Service, task db.Task) (db.Task, error) {
		return tasks.SetCompleted(task.ID, completed)
	}
}

func (h *webHarness) chatThreadCount(want int) {
	h.R.Eventually(func() bool {
		threads, err := h.Chat.Threads()
		return err == nil && len(threads) == want
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *webHarness) chatThreadLabel(id int64, want string) {
	h.R.Eventually(func() bool {
		thread, err := h.Chat.Thread(id)
		return err == nil && h.Chat.ThreadLabel(thread) == want
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *webHarness) eventuallyPresent(selector string) {
	h.R.Eventuallyf(func() bool {
		_, err := h.Page.Element(selector)
		return err == nil
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval, "%s should be present", selector)
}

func (h *webHarness) hover(selector string) {
	h.Page.MustElement(selector).MustHover()
}

func (h *webHarness) pathEventuallyEquals(expected string) {
	h.R.Eventuallyf(func() bool {
		result, err := h.Page.Eval(`() => location.pathname`)
		return err == nil && result.Value.Str() == expected
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval, "path should equal %q", expected)
}

func (h *webHarness) seedChat(title, label string) {
	_, err := h.Chat.CreateThreadWithLabel("", "", label, title)
	h.R.NoError(err)
}

func (h *webHarness) seedTask(title string, options ...webTaskOption) {
	task, err := h.Tasks.Create(title)
	h.R.NoError(err)
	for _, option := range options {
		task, err = option(h.Tasks, task)
		h.R.NoError(err)
	}
}

func (h *webHarness) taskEventually(id int64, match func(db.Task) bool) {
	h.R.Eventually(func() bool {
		task, err := h.Tasks.Get(id)
		return err == nil && match(task)
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *webHarness) taskEventuallyDeleted(id int64) {
	h.R.Eventually(func() bool {
		_, err := h.Tasks.Get(id)
		return db.IsTaskNotFound(err)
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}
