package flow

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/inbox"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ts"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/housecat-inc/scratch/testkit"
)

type Harness struct {
	*testkit.Harness
	Clock *ts.MockTime
	Store *db.DB
	Svc   *chat.Service
}

type Case = testkit.BrowserCase[*Harness]

type Step = testkit.BrowserStep[*Harness]

func run(t *testing.T, cases []Case) {
	testkit.RunBrowserCases(t, cases, testkit.BrowserCaseRunner[*Harness]{
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

			path := filepath.Join(t.TempDir(), "scratch.db")
			store, err := db.New(path)
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })

			workflows, err := workflow.New(path)
			kit.R.NoError(err)

			svc := chat.NewService(store, chat.EchoAgent{Delay: 10 * time.Millisecond}, slog.New(logs))
			flows := New(Deps{DBOS: workflows.Ctx(), Extract: HeuristicExtractor(), Log: slog.New(logs), Publish: svc.Publish, Store: store, Tasks: store})
			svc.RegisterAgent("contact", flows.Agent())
			svc.SetResolver(flows)
			kit.R.NoError(workflows.Launch())
			t.Cleanup(func() {
				svc.Close()
				workflows.Close()
			})

			mux := http.NewServeMux()
			mux.Handle("/chat/", chat.NewServer(svc, slog.New(logs)).Handler())
			mux.Handle("/", inbox.NewServer(todo.NewService(store), svc, slog.New(logs)).Handler())

			return &Harness{
				Harness: testkit.NewHarnessWithT(t, kit, mux),
				Clock:   clock,
				Store:   store,
				Svc:     svc,
			}
		},
	})
}

var Click = testkit.ClickStep[*Harness]
var TextContains = testkit.TextContainsStep[*Harness]
var Type = testkit.TypeStep[*Harness]
var Visible = testkit.VisibleStep[*Harness]

func SeedPendingForm(prompt string) Step {
	return func(t *testing.T, h *Harness) {
		thread, err := h.Svc.CreateThread("contact", "")
		h.R.NoError(err)
		_, err = h.Svc.Send(thread.ID, prompt)
		h.R.NoError(err)
		h.R.Eventually(func() bool {
			view, err := h.Svc.View(thread.ID)
			return err == nil && len(view.Forms) > 0
		}, 10*time.Second, 20*time.Millisecond)
	}
}

func Tasks(titles ...string) Step {
	return func(t *testing.T, h *Harness) {
		h.R.Eventually(func() bool {
			tasks, err := h.Store.ListTasks()
			if err != nil || len(tasks) != len(titles) {
				return false
			}
			for i, task := range tasks {
				if task.Title != titles[i] {
					return false
				}
			}
			return true
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func TestContactIntakeBrowser(t *testing.T) {
	run(t, []Case{
		{
			Name: "elicits a contact form and saves a todo on accept",
			Path: "/inbox/workflows/1",
			Seed: []Step{
				SeedPendingForm("Add Jane Doe jane@example.com from ACME"),
			},
			Act: []Step{
				Visible("#elicit-form"),
				Type("[name=f_company]", "ACME"),
				Type("[name=f_notes]", "Met at the conference"),
				Click("#elicit-accept"),
			},
			Assert: []Step{
				TextContains("#chat-messages", "Added todo #1: Follow up with Jane Doe"),
				TextContains("#chat-messages", "jane@example.com"),
				Tasks("Follow up with Jane Doe <jane@example.com>"),
			},
		},
		{
			Name: "declining the form discards the draft",
			Path: "/inbox/workflows/1",
			Seed: []Step{
				SeedPendingForm("met bob@example.com"),
			},
			Act: []Step{
				Visible("#elicit-form"),
				Click("#elicit-decline"),
			},
			Assert: []Step{
				TextContains("#chat-messages", "discarded the contact draft"),
				Tasks(),
			},
		},
		{
			Name: "renders a pending review form",
			Path: "/inbox/workflows/1",
			Seed: []Step{
				SeedPendingForm("Add Jane Doe jane@example.com from ACME"),
			},
			Assert: []Step{
				Visible("#elicit-form"),
				TextContains("#chat-messages", "Review the contact before I save it."),
			},
		},
	})
}
