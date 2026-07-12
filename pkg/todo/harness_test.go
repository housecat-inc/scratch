package todo

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
	Act         []Step
	Assert      []Step
	Console     []string
	Name        string
	Path        string
	Screenshots []testkit.Screenshot
	Seed        []Step
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

			svc := NewService(store)
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

			if len(tc.Screenshots) > 0 {
				h.MatchScreenshots(tc.Screenshots, "begin", tc.Path)
			} else {
				h.Load(tc.Path)
			}

			logs.Reset()

			for _, step := range tc.Act {
				step(t, h)
			}
			for _, step := range tc.Assert {
				step(t, h)
			}

			if len(tc.Screenshots) > 0 {
				h.MatchScreenshots(tc.Screenshots, "end", tc.Path)
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

func ElementAbsent(selector string) Step {
	return func(t *testing.T, h *Harness) { h.ElementAbsent(selector) }
}

func ClassContains(selector, expected string) Step {
	return func(t *testing.T, h *Harness) { h.ElementAttributeContains(selector, "class", expected) }
}

func Log(line string) Step {
	return func(t *testing.T, h *Harness) { h.R.Contains(h.Logs.Lines(), line) }
}

func Metric(subset string, count int) Step {
	return func(t *testing.T, h *Harness) { h.A.Equal(count, h.Logs.Count(subset)) }
}

func Press(selector, key string) Step {
	return func(t *testing.T, h *Harness) { h.Press(selector, key) }
}

func SeedTasks(titles ...string) Step {
	return func(t *testing.T, h *Harness) {
		for _, title := range titles {
			_, err := h.Svc.Create(title)
			h.R.NoError(err)
			h.Clock.Advance(time.Minute)
		}
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
