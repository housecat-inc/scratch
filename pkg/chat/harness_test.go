package chat

import (
	"log/slog"
	"slices"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ts"
	"github.com/housecat-inc/scratch/testkit"
)

type Harness struct {
	*testkit.Harness
	Clock *ts.MockTime
	Logs  *testkit.LogRecorder
	Svc   *Service
}

type Step func(t *testing.T, h *Harness)

type Case struct {
	Act     []Step
	Agent   Agent
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

			store, err := db.New(":memory:")
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })

			agent := tc.Agent
			if agent == nil {
				agent = EchoAgent{Delay: 10 * time.Millisecond}
			}
			svc := NewService(store, agent, slog.New(logs))
			t.Cleanup(svc.Close)
			handler := NewServer(svc, slog.New(logs)).Handler()

			h := &Harness{
				Harness: testkit.NewHarnessWithT(t, kit, handler),
				Clock:   clock,
				Logs:    logs,
				Svc:     svc,
			}

			for _, step := range tc.Seed {
				step(t, h)
			}

			h.Load(tc.Path)

			logs.Reset()

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

func ClassContains(selector, expected string) Step {
	return func(t *testing.T, h *Harness) { h.ElementAttributeContains(selector, "class", expected) }
}

func Click(selector string) Step {
	return func(t *testing.T, h *Harness) { h.Click(selector) }
}

func Log(line string) Step {
	return func(t *testing.T, h *Harness) { h.R.Contains(h.Logs.Lines(), line) }
}

func Press(selector, key string) Step {
	return func(t *testing.T, h *Harness) { h.Press(selector, key) }
}

func SeedExchange(threadID int64, prompt string) Step {
	return func(t *testing.T, h *Harness) {
		_, err := h.Svc.Send(threadID, prompt)
		h.R.NoError(err)
		h.R.Eventually(func() bool {
			view, err := h.Svc.View(threadID)
			return err == nil && !view.Streaming
		}, 5*time.Second, 10*time.Millisecond)
		h.Clock.Advance(time.Minute)
	}
}

func SeedThread(title string) Step {
	return func(t *testing.T, h *Harness) {
		_, err := h.Svc.CreateThread("", title)
		h.R.NoError(err)
		h.Clock.Advance(time.Minute)
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
