package todo

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/testkit"
)

type webHarness struct {
	*testkit.Harness
	Tasks *Service
}

type webStep = testkit.BrowserStep[*webHarness]

func runWebBrowser(t *testing.T, cases []testkit.BrowserCase[*webHarness]) {
	t.Helper()
	testkit.RunBrowserCases(t, cases, testkit.BrowserCaseRunner[*webHarness]{
		ConsoleErrors: func(h *webHarness) []string {
			return h.Console.Errors()
		},
		Load: func(h *webHarness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*webHarness]) *webHarness {
			store, err := db.New(":memory:")
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })

			tasks := NewService(store)
			mux := http.NewServeMux()
			mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			mux.Handle("/", NewWebServer(tasks, slog.Default()).Handler())

			return &webHarness{
				Harness: testkit.NewHarnessWithT(t, kit, mux),
				Tasks:   tasks,
			}
		},
	})
}

func TestTodoWebBrowser(t *testing.T) {
	runWebBrowser(t, []testkit.BrowserCase[*webHarness]{
		{
			Act: []webStep{
				testkit.TypeStep[*webHarness](".todo-new-input", "buy milk"),
				testkit.ClickStep[*webHarness](".todo-create-form button"),
			},
			Assert: []webStep{
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "buy milk"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "Active"),
				webElementAbsent("section.mail-card"),
			},
			Name: "creates a task",
			Path: "/",
		},
		{
			Assert: []webStep{
				testkit.TextContainsStep[*webHarness](".mail-mainbar", "Inbox"),
				testkit.TextContainsStep[*webHarness](`[data-id="1"]`, "active one"),
				testkit.VisibleStep[*webHarness](`[data-id="1"] form[action="/tasks/1/done"] button`),
				testkit.VisibleStep[*webHarness](`[data-id="1"] button[aria-label="Archive"]`),
				webElementAbsent(".mail-reader-head"),
				webElementAbsent(`[data-id="2"]`),
				webElementAbsent(`[data-id="3"]`),
			},
			Name: "inbox lists active tasks",
			Path: "/",
			Seed: []webStep{
				seedWebTask("active one"),
				seedWebTask("done one", webTaskCompleted(true)),
				seedWebTask("archived one", webTaskArchived(true)),
			},
		},
		{
			Act: []webStep{
				testkit.ClickStep[*webHarness](".gm-row-link"),
			},
			Assert: []webStep{
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "open task"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "Active"),
				webElementAbsent("section.mail-card"),
			},
			Name: "opens a task page from the inbox row",
			Path: "/",
			Seed: []webStep{
				seedWebTask("open task"),
			},
		},
		{
			Assert: []webStep{
				testkit.TextContainsStep[*webHarness](".mail-mainbar", "Tasks"),
				testkit.TextContainsStep[*webHarness](`[data-id="1"]`, "active one"),
				testkit.TextContainsStep[*webHarness](`[data-id="2"]`, "done one"),
				testkit.TextContainsStep[*webHarness](`[data-id="3"]`, "archived one"),
				testkit.VisibleStep[*webHarness](`[data-id="3"] button[aria-label="Unarchive"]`),
				testkit.VisibleStep[*webHarness](`[data-id="3"] button[aria-label="Delete"]`),
				webElementAbsent(".mail-reader-head"),
			},
			Name: "tasks lists all tasks with archived actions",
			Path: "/tasks",
			Seed: []webStep{
				seedWebTask("active one"),
				seedWebTask("done one", webTaskCompleted(true)),
				seedWebTask("archived one", webTaskArchived(true)),
			},
		},
		{
			Act: []webStep{
				webHover(`[data-id="1"]`),
				testkit.ClickStep[*webHarness](`[data-id="1"] button[aria-label="Archive"]`),
				webLoad("/tasks"),
				webHover(`[data-id="1"]`),
				testkit.ClickStep[*webHarness](`[data-id="1"] button[aria-label="Unarchive"]`),
			},
			Assert: []webStep{
				webTaskArchivedState(1, false),
				testkit.VisibleStep[*webHarness](`[data-id="1"] button[aria-label="Archive"]`),
			},
			Name: "archives and unarchives from list actions",
			Path: "/",
			Seed: []webStep{
				seedWebTask("archive me"),
			},
		},
		{
			Act: []webStep{
				webHover(`[data-id="1"]`),
				testkit.ClickStep[*webHarness](`[data-id="1"] button[aria-label="Delete"]`),
				webTaskPresent(1),
				testkit.VisibleStep[*webHarness](`[data-id="1"] [data-trash-submit]`),
				testkit.ClickStep[*webHarness](`[data-id="1"] [data-trash-submit]`),
			},
			Assert: []webStep{
				webTaskDeleted(1),
				webElementAbsent(`[data-id="1"]`),
			},
			Name: "deletes archived tasks from the tasks list",
			Path: "/tasks",
			Seed: []webStep{
				seedWebTask("remove me", webTaskArchived(true)),
			},
		},
		{
			Act: []webStep{
				testkit.ClickStep[*webHarness](`.mail-reader-head form[action="/tasks/1/done"] button`),
			},
			Assert: []webStep{
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "Done"),
				webTaskCompletedState(1, true),
			},
			Name: "toggles completion",
			Path: "/tasks/1",
			Seed: []webStep{
				seedWebTask("draft"),
			},
		},
		{
			Act: []webStep{
				testkit.TypeStep[*webHarness](`[name="description"]`, "Call customer"),
				testkit.ClickStep[*webHarness](".mail-save-btn"),
			},
			Assert: []webStep{
				webTaskDescription(1, "Call customer"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "draft"),
			},
			Name: "updates description",
			Path: "/tasks/1",
			Seed: []webStep{
				seedWebTask("draft"),
			},
		},
	})
}

type webTaskOption func(*Service, db.Task) (db.Task, error)

func seedWebTask(title string, options ...webTaskOption) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		task, err := h.Tasks.Create(title)
		h.R.NoError(err)
		for _, option := range options {
			task, err = option(h.Tasks, task)
			h.R.NoError(err)
		}
	}
}

func webElementAbsent(selector string) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.ElementAbsent(selector)
	}
}

func webHover(selector string) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.Page.MustElement(selector).MustHover()
	}
}

func webLoad(path string) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.Load(path)
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

func webTaskArchivedState(id int64, want bool) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.R.Eventually(func() bool {
			task, err := h.Tasks.Get(id)
			return err == nil && task.Archived == want
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func webTaskCompletedState(id int64, want bool) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.R.Eventually(func() bool {
			task, err := h.Tasks.Get(id)
			return err == nil && task.Completed == want
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func webTaskDeleted(id int64) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.R.Eventually(func() bool {
			_, err := h.Tasks.Get(id)
			return db.IsTaskNotFound(err)
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func webTaskPresent(id int64) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.R.Eventually(func() bool {
			_, err := h.Tasks.Get(id)
			return err == nil
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func webTaskDescription(id int64, want string) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		h.R.Eventually(func() bool {
			task, err := h.Tasks.Get(id)
			return err == nil && task.Description == want
		}, 5*time.Second, 50*time.Millisecond)
	}
}
