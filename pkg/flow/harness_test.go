package flow

import (
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
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

type Step func(t *testing.T, h *Harness)

type Case struct {
	Act     []Step
	Assert  []Step
	Console []string
	Name    string
	Path    string
	Seed    []Step
}

func run(t *testing.T, cases []Case) {
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
			defer restore()

			kit := testkit.New(t)
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

			handler := chat.NewServer(svc, slog.New(logs)).Handler()

			h := &Harness{
				Harness: testkit.NewHarnessWithT(t, kit, handler),
				Clock:   clock,
				Store:   store,
				Svc:     svc,
			}

			for _, step := range tc.Seed {
				step(t, h)
			}

			h.Load(tc.Path)

			for _, step := range tc.Act {
				step(t, h)
			}
			for _, step := range tc.Assert {
				step(t, h)
			}

			for _, msg := range h.Console.Errors() {
				if !slices.Contains(tc.Console, msg) {
					t.Errorf("unexpected console error: %s", msg)
				}
			}
		})
	}
}

func Click(selector string) Step {
	return func(t *testing.T, h *Harness) { h.Click(selector) }
}

func Press(selector, key string) Step {
	return func(t *testing.T, h *Harness) { h.Press(selector, key) }
}

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

func TextContains(selector, expected string) Step {
	return func(t *testing.T, h *Harness) { h.ElementTextContains(selector, expected) }
}

func Type(selector, text string) Step {
	return func(t *testing.T, h *Harness) { h.Type(selector, text) }
}

func Visible(selector string) Step {
	return func(t *testing.T, h *Harness) { h.ElementVisible(selector) }
}

func TestContactIntakeBrowser(t *testing.T) {
	run(t, []Case{
		{
			Name: "elicits a contact form and saves a todo on accept",
			Path: "/chat",
			Act: []Step{
				Click("#new-thread-contact"),
				TextContains("#thread-agent", "contact"),
				Type("#chat-input", "Add Jane Doe jane@example.com from ACME"),
				Press("#chat-input", "Enter"),
				Visible("#elicit-form"),
				Type("[name=f_company]", "ACME"),
				Type("[name=f_notes]", "Met at the conference"),
				Click("#elicit-accept"),
			},
			Assert: []Step{
				TextContains("#chat-messages", "Added todo #1: Follow up with Jane Doe <jane@example.com>"),
				Tasks("Follow up with Jane Doe <jane@example.com>"),
			},
		},
		{
			Name: "declining the form discards the draft",
			Path: "/chat",
			Act: []Step{
				Click("#new-thread-contact"),
				Type("#chat-input", "met bob@example.com"),
				Press("#chat-input", "Enter"),
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
			Path: "/chat/1",
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
