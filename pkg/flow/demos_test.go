package flow

import (
	"testing"
	"time"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/require"
)

func TestCountdownWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "counts down through each tick then lifts off",
			Act:  []Step{Start("countdown", "countdown-1")},
			Assert: []Step{
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
	})
}

func TestFanoutWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "runs jobs on a queue then sums the squares",
			Act:  []Step{Start("fan-out", "fanout-1")},
			Assert: []Step{
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
	})
}

func TestStreamWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "streams log lines then closes",
			Act:  []Step{Start("stream", "stream-1")},
			Assert: []Step{
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
	})
}

func TestDeployWorkflow(t *testing.T) {
	runCases(t, []testkit.Case[*Harness]{
		{
			Name: "publishes version and status events",
			Act:  []Step{Start("deploy", "deploy-1")},
			Assert: []Step{
				ExpectResult("Deployed v1.4.2"),
				ExpectStepCount(1),
				ExpectLastStep(func(h *Harness, s StepView) {
					h.Equal("Live status", s.Title)
					h.R.Contains(s.Detail, "version: v1.4.2")
					h.R.Contains(s.Detail, "status: Live")
				}),
			},
		},
	})
}

func TestFanoutEnqueuesChildWorkflows(t *testing.T) {
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
}
