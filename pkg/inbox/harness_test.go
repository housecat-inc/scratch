package inbox

import (
	"context"
	"log/slog"
	"net/http"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ts"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/housecat-inc/scratch/testkit"
)

type Harness struct {
	*testkit.Harness
	Chat  *chat.Service
	Clock *ts.MockTime
	Flows *flow.Engine
	Logs  *testkit.LogRecorder
	Store *db.DB
	Tasks *todo.Service
}

type Step = testkit.BrowserStep[*Harness]

type stubUpdater struct{}

func (stubUpdater) Version(context.Context) (string, error) { return "claude 2.1.0", nil }
func (stubUpdater) Update(context.Context) (string, error)  { return "Updated to claude 2.2.0", nil }

type stubDrafter struct{}

func (stubDrafter) Context(context.Context) (string, error) {
	return "Commits:\nabc123 Add a widget", nil
}

func (stubDrafter) Draft(context.Context, string) (flow.PullRequest, error) {
	return flow.PullRequest{Body: "- Add a widget", Title: "Add a widget"}, nil
}

func runBrowser(t *testing.T, cases []testkit.BrowserCase[*Harness]) {
	testkit.RunBrowserCases(t, cases, testkit.BrowserCaseRunner[*Harness]{
		BeforeAct: func(h *Harness) {
			h.Logs.Reset()
		},
		ConsoleErrors: func(h *Harness) []string {
			return h.Console.Errors()
		},
		Load: func(h *Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*Harness]) *Harness {
			clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
			t.Cleanup(restore)
			logs := testkit.NewLogRecorder(t, kit.Artifacts)

			store, err := db.New(":memory:")
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })

			workflows, err := workflow.New(t.TempDir() + "/workflows.db")
			kit.R.NoError(err)
			flows := flow.New(flow.Deps{
				DBOS:    workflows.Ctx(),
				Drafter: stubDrafter{},
				Greeter: flow.HeuristicGreeter(),
				Log:     slog.New(logs),
				Updater: stubUpdater{},
			})
			kit.R.NoError(workflows.Launch())
			t.Cleanup(func() { workflows.Close() })

			chatSvc := chat.NewService(store, chat.EchoAgent{Delay: 10 * time.Millisecond}, slog.New(logs))
			t.Cleanup(chatSvc.Close)
			taskSvc := todo.NewService(store)

			mux := http.NewServeMux()
			mux.Handle("/chat/", chat.NewServer(chatSvc, slog.New(logs)).Handler())
			mux.Handle("/", NewServer(taskSvc, chatSvc, flows, slog.New(logs)).Handler())

			return &Harness{
				Harness: testkit.NewHarnessWithT(t, kit, mux),
				Chat:    chatSvc,
				Clock:   clock,
				Flows:   flows,
				Logs:    logs,
				Store:   store,
				Tasks:   taskSvc,
			}
		},
	})
}

var Click = testkit.ClickStep[*Harness]
var ElementAbsent = func(selector string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.ElementAbsent(selector)
	}
}
var Press = testkit.PressStep[*Harness]
var TextContains = testkit.TextContainsStep[*Harness]
var Type = testkit.TypeStep[*Harness]
var Visible = testkit.VisibleStep[*Harness]

func Load(path string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.Load(path)
	}
}

type TaskOption func(*todo.Service, db.Task) (db.Task, error)

func SeedTask(title string, options ...TaskOption) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		task, err := h.Tasks.Create(title)
		h.R.NoError(err)
		for _, option := range options {
			task, err = option(h.Tasks, task)
			h.R.NoError(err)
		}
		h.Clock.Advance(time.Minute)
	}
}

func TaskArchived(archived bool) TaskOption {
	return func(tasks *todo.Service, task db.Task) (db.Task, error) {
		return tasks.SetArchived(task.ID, archived)
	}
}

func TaskCompleted(completed bool) TaskOption {
	return func(tasks *todo.Service, task db.Task) (db.Task, error) {
		return tasks.SetCompleted(task.ID, completed)
	}
}

func TaskDescription(id int64, want string) Step {
	return func(t *testing.T, h *Harness) {
		t.Helper()
		h.R.Eventually(func() bool {
			task, err := h.Tasks.Get(id)
			return err == nil && task.Description == want
		}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
	}
}
