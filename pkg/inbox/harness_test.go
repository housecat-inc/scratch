package inbox

import (
	"context"
	"log/slog"
	"net/http"
	"slices"
	"strings"
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
	Chat    *chat.Service
	Clock   *ts.MockTime
	Flows   *flow.Engine
	Logs    *testkit.LogRecorder
	Store   *db.DB
	Tasks   *todo.Service
	console []string
	t       *testing.T
}

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

type TaskOption func(*todo.Service, db.Task) (db.Task, error)

func newHarness(t *testing.T) *Harness {
	t.Helper()
	kit := testkit.New(t)
	clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	t.Cleanup(restore)
	logs := testkit.NewLogRecorder(t, kit.Artifacts)

	store, err := db.New(":memory:")
	kit.R.NoError(err)
	t.Cleanup(func() { store.Close() })

	workflows, err := workflow.New(t.TempDir() + "/workflows.db")
	kit.R.NoError(err)
	flows := flow.New(flow.Deps{
		CountdownStep: 400 * time.Millisecond,
		DBOS:          workflows.Ctx(),
		Drafter:       stubDrafter{},
		Greeter:       flow.HeuristicGreeter(),
		Log:           slog.New(logs),
		StageStep:     700 * time.Millisecond,
		Updater:       stubUpdater{},
	})
	kit.R.NoError(workflows.Launch())
	t.Cleanup(func() { workflows.Close() })

	chatSvc := chat.NewService(store, chat.EchoAgent{Delay: 10 * time.Millisecond}, slog.New(logs))
	t.Cleanup(chatSvc.Close)
	taskSvc := todo.NewService(store)

	mux := http.NewServeMux()
	mux.Handle("/chat/", chat.NewServer(chatSvc, slog.New(logs)).Handler())
	codex := chat.NewCodexServer(chatSvc)
	mux.Handle("/codex", codex.Handler())
	mux.Handle("/codex/", codex.Handler())
	mux.Handle("/", codex.Middleware(NewServer(taskSvc, chatSvc, flows, slog.New(logs)).Handler()))

	h := &Harness{
		Harness: testkit.NewHarnessWithT(t, kit, mux),
		Chat:    chatSvc,
		Clock:   clock,
		Flows:   flows,
		Logs:    logs,
		Store:   store,
		Tasks:   taskSvc,
		t:       t,
	}
	t.Cleanup(h.checkConsole)
	return h
}

func (h *Harness) AwaitWorkflowRunning(running bool) {
	h.t.Helper()
	h.R.Eventually(func() bool {
		threads, err := h.Chat.Threads()
		if err != nil || len(threads) == 0 {
			return false
		}
		id := h.Chat.ThreadWorkflowID(threads[0])
		if id == "" {
			return false
		}
		run, err := h.Flows.Run(id)
		return err == nil && run.Running() == running
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *Harness) ChatThreadCount(want int) {
	h.t.Helper()
	h.R.Eventually(func() bool {
		threads, err := h.Chat.Threads()
		return err == nil && len(threads) == want
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *Harness) ElementEventuallyPresent(selector string) {
	h.t.Helper()
	h.R.Eventuallyf(func() bool {
		_, err := h.Page.Element(selector)
		return err == nil
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval, "%s should be present", selector)
}

func (h *Harness) EvalBool(js string, args ...any) bool {
	v, err := h.Page.Eval(js, args...)
	return err == nil && v.Value.Bool()
}

func (h *Harness) EventuallyTrue(js string, args ...any) {
	h.t.Helper()
	h.R.Eventually(func() bool {
		return h.EvalBool(js, args...)
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *Harness) Hover(selector string) {
	h.t.Helper()
	h.Page.MustElement(selector).MustHover()
}

func (h *Harness) Open(path string) {
	h.t.Helper()
	h.Load(path)
	h.Logs.Reset()
}

func (h *Harness) PathEventuallyEquals(expected string) {
	h.t.Helper()
	h.R.Eventuallyf(func() bool {
		result, err := h.Page.Eval(`() => location.pathname`)
		return err == nil && result.Value.Str() == expected
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval, "path should equal %q", expected)
}

func (h *Harness) SeedAssistantMessage(body string, events ...[2]string) {
	h.t.Helper()
	msg, err := h.Store.AddMessage(db.NewMessage{
		Author:   "agent:echo",
		Body:     body,
		Role:     db.MessageRoleAssistant,
		ThreadID: 1,
	})
	h.R.NoError(err)
	for _, ev := range events {
		_, err := h.Store.AddMessageEvent(msg.ID, ev[0], ev[1])
		h.R.NoError(err)
	}
}

func (h *Harness) SeedChatThread(title string) {
	h.t.Helper()
	_, err := h.Chat.CreateThread("", title)
	h.R.NoError(err)
}

func (h *Harness) SeedRailThread(messages ...string) int64 {
	h.t.Helper()
	thread, err := h.Chat.CreateThread("", "")
	h.R.NoError(err)
	for _, message := range messages {
		_, err = h.Chat.Send(thread.ID, message)
		h.R.NoError(err)
	}
	return thread.ID
}

func (h *Harness) SeedRailThreadMessage(body string) {
	h.t.Helper()
	_, err := h.Store.AddMessage(db.NewMessage{
		Author:   "user",
		Body:     body,
		Role:     db.MessageRoleUser,
		ThreadID: 1,
	})
	h.R.NoError(err)
}

func (h *Harness) SeedSchedule() {
	h.t.Helper()
	h.R.NoError(h.Flows.EnsureSchedules())
}

func (h *Harness) SeedScheduleRuns(n int) {
	h.t.Helper()
	runIDs := make([]string, 0, n)
	for range n {
		runID, err := h.Flows.TriggerSchedule("heartbeat")
		h.R.NoError(err)
		runIDs = append(runIDs, runID)
	}
	for _, runID := range runIDs {
		h.Flows.Await(runID, 5*time.Second)
	}
}

func (h *Harness) SeedStreamingAssistant(body string) {
	h.t.Helper()
	_, err := h.Store.AddMessage(db.NewMessage{
		Author:   "agent:echo",
		Body:     body,
		Role:     db.MessageRoleAssistant,
		Status:   db.MessageStatusStreaming,
		ThreadID: 1,
	})
	h.R.NoError(err)
}

func (h *Harness) SeedTask(title string, options ...TaskOption) {
	h.t.Helper()
	task, err := h.Tasks.Create(title)
	h.R.NoError(err)
	for _, option := range options {
		task, err = option(h.Tasks, task)
		h.R.NoError(err)
	}
	h.Clock.Advance(time.Minute)
}

func (h *Harness) SelectOption(selector, value string) {
	h.t.Helper()
	ok, err := h.Page.Eval(`(selector, value) => {
		const select = document.querySelector(selector);
		if (!select) return false;
		select.value = value;
		select.dispatchEvent(new Event("change", { bubbles: true }));
		return true;
	}`, selector, value)
	h.R.NoError(err)
	h.R.True(ok.Value.Bool())
}

func (h *Harness) TaskCount(want int) {
	h.t.Helper()
	h.R.Eventually(func() bool {
		tasks, err := h.Tasks.All()
		return err == nil && len(tasks) == want
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *Harness) TaskDescription(id int64, want string) {
	h.t.Helper()
	h.R.Eventually(func() bool {
		task, err := h.Tasks.Get(id)
		return err == nil && task.Description == want
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval)
}

func (h *Harness) TextEventuallyContains(selector, expected string) {
	h.t.Helper()
	h.R.Eventuallyf(func() bool {
		el, err := h.Page.Element(selector)
		if err != nil {
			return false
		}
		text, err := el.Text()
		return err == nil && strings.Contains(text, expected)
	}, testkit.BrowserWaitTimeout, testkit.BrowserPollInterval, "%s text should contain %q", selector, expected)
}

func (h *Harness) checkConsole() {
	if h.t.Failed() {
		return
	}
	for _, msg := range h.Console.Errors() {
		if !slices.Contains(h.console, msg) {
			h.t.Errorf("unexpected console error: %s", msg)
		}
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
