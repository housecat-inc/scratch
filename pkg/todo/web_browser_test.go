package todo

import (
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
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
				testkit.TextContainsStep[*webHarness](".gm-row", "buy milk"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "buy milk"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "Active"),
			},
			Name: "creates a task",
			Path: "/",
		},
		{
			Act: []webStep{
				testkit.ClickStep[*webHarness](`.todo-reader-head form[action="/tasks/1/done"] button`),
			},
			Assert: []webStep{
				testkit.ClassContainsStep[*webHarness](`[data-id="1"]`, "done"),
				testkit.TextContainsStep[*webHarness](".mail-reader-title", "Done"),
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

func seedWebTask(title string) webStep {
	return func(t *testing.T, h *webHarness) {
		t.Helper()
		_, err := h.Tasks.Create(title)
		h.R.NoError(err)
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
