package flow

import (
	"testing"
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/stretchr/testify/require"
)

func TestCountdownWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "counts down through each tick then lifts off",
			act:  []Step{Start("countdown", "countdown-1")},
			assert: []Step{
				ExpectResult("🚀 Liftoff!"),
				ExpectStepCount(4),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal(KindResponse, s.Kind)
					h.Equal("T-minus 3", s.Title)
					h.Equal("3…", s.Detail)
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Liftoff", s.Title)
					h.Equal("🚀 Liftoff!", s.Detail)
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}

func TestDeployWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "publishes version and status events",
			act:  []Step{Start("deploy", "deploy-1")},
			assert: []Step{
				ExpectResult("Deployed v1.4.2"),
				ExpectStepCount(2),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal("Release v1.4.2 — available immediately", s.Title)
					h.Equal(StepDone, s.Status)
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Live status", s.Title)
					h.Equal("Live", s.Detail)
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}

func TestFanoutWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "runs jobs on a queue then sums the squares",
			act:  []Step{Start("fan-out", "fanout-1")},
			assert: []Step{
				ExpectStepCount(5),
				ExpectStep(0, func(h *Harness, s StepView) {
					h.Equal(KindResponse, s.Kind)
					h.Equal("Job 1", s.Title)
					h.Equal("1² = 1", s.Detail)
				}),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Summary", s.Title)
					h.R.Contains(s.Detail, "sum of squares = 30")
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}

	t.Run("enqueues one child workflow per job", func(t *testing.T) {
		r := require.New(t)
		e := newEngine(t)
		r.NoError(e.Start("fan-out", "fanout-children"))
		e.Await("fanout-children", 5*time.Second)

		run, err := e.Run("fanout-children")
		r.NoError(err)
		r.True(run.Done())

		children, err := dbos.ListWorkflows(e.ctx, dbos.WithHasParent(true))
		r.NoError(err)
		r.Len(children, e.fanoutJobs)
	})
}

func TestStreamWorkflow(t *testing.T) {
	tests := []flowCase{
		{
			name: "streams log lines then closes",
			act:  []Step{Start("stream", "stream-1")},
			assert: []Step{
				ExpectResult("Streamed 5 log lines"),
				ExpectStepCount(1),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Log stream", s.Title)
					h.Equal(StepDone, s.Status)
					h.R.Contains(s.Detail, "Connecting")
					h.R.Contains(s.Detail, "Done ✓")
				}),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) { runFlowCase(t, tt) })
	}
}
