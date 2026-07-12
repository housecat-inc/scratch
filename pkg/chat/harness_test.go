package chat

import (
	"log/slog"
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

type Case struct {
	Act     []Step
	Agent   Agent
	Assert  []Step
	Console []string
	Name    string
	Path    string
	Seed    []Step
}

type Step = testkit.BrowserStep[*Harness]

func run(t *testing.T, cases []Case) {
	agents := map[string]Agent{}
	baseCases := make([]testkit.BrowserCase[*Harness], 0, len(cases))
	for _, tc := range cases {
		agents[tc.Name] = tc.Agent
		baseCases = append(baseCases, testkit.BrowserCase[*Harness]{
			Act:     tc.Act,
			Assert:  tc.Assert,
			Console: tc.Console,
			Name:    tc.Name,
			Path:    tc.Path,
			Seed:    tc.Seed,
		})
	}
	testkit.RunBrowserCases(t, baseCases, testkit.BrowserCaseRunner[*Harness]{
		BeforeAct: func(h *Harness) {
			h.Logs.Reset()
		},
		ConsoleErrors: func(h *Harness) []string {
			return h.Console.Errors()
		},
		Load: func(h *Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, tc testkit.BrowserCase[*Harness]) *Harness {
			clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
			t.Cleanup(restore)
			logs := testkit.NewLogRecorder(t, kit.Artifacts)

			store, err := db.New(":memory:")
			kit.R.NoError(err)
			t.Cleanup(func() { store.Close() })

			agent := agents[tc.Name]
			if agent == nil {
				agent = EchoAgent{Delay: 10 * time.Millisecond}
			}
			svc := NewService(store, agent, slog.New(logs))
			t.Cleanup(svc.Close)
			handler := NewServer(svc, slog.New(logs)).Handler()

			return &Harness{
				Harness: testkit.NewHarnessWithT(t, kit, handler),
				Clock:   clock,
				Logs:    logs,
				Svc:     svc,
			}
		},
	})
}

var ClassContains = testkit.ClassContainsStep[*Harness]
var Click = testkit.ClickStep[*Harness]
var Press = testkit.PressStep[*Harness]
var TextContains = testkit.TextContainsStep[*Harness]
var Type = testkit.TypeStep[*Harness]
var Visible = testkit.VisibleStep[*Harness]

func Log(line string) Step {
	return func(t *testing.T, h *Harness) { h.R.Contains(h.Logs.Lines(), line) }
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
