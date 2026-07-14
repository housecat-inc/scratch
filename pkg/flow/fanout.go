package flow

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	defaultFanoutJobs  = 4
	fanoutQueue        = "demo-fanout"
	fanoutQueueWorkers = 3
)

type FanoutInput struct{}

type JobInput struct {
	N int `json:"n"`
}

func (e *Engine) fanout(ctx dbos.DBOSContext, _ FanoutInput) (string, error) {
	jobs := e.fanoutJobs
	if _, err := dbos.RunAsStep(ctx, func(context.Context) (stepBegin, error) {
		return stepBegin{Input: fmt.Sprintf("%d jobs", jobs), Name: "dispatch", Title: "Dispatch jobs"}, nil
	}, dbos.WithStepName("begin/dispatch")); err != nil {
		return "", errors.Wrap(err, "begin dispatch")
	}

	handles := make([]dbos.WorkflowHandle[int], 0, jobs)
	for i := 1; i <= jobs; i++ {
		handle, err := dbos.RunWorkflow(ctx, e.job, JobInput{N: i}, dbos.WithQueue(fanoutQueue))
		if err != nil {
			return "", errors.Wrapf(err, "enqueue job %d", i)
		}
		handles = append(handles, handle)
	}

	total := 0
	for i, handle := range handles {
		square, err := handle.GetResult()
		if err != nil {
			return "", errors.Wrapf(err, "job %d", i+1)
		}
		total += square
		n, result := i+1, square
		if _, err := act(ctx, fmt.Sprintf("respond/job-%d", n), fmt.Sprintf("Job %d", n), fmt.Sprintf("n=%d", n), func(context.Context) (string, error) {
			return fmt.Sprintf("%d² = %d", n, result), nil
		}); err != nil {
			return "", errors.Wrapf(err, "record job %d", n)
		}
	}

	summary := fmt.Sprintf("Ran %d jobs on the %q queue (%d workers); sum of squares = %d", jobs, fanoutQueue, fanoutQueueWorkers, total)
	if _, err := act(ctx, "respond/summary", "Summary", "", func(context.Context) (string, error) {
		return summary, nil
	}); err != nil {
		return "", errors.Wrap(err, "summary")
	}
	return summary, nil
}

func (e *Engine) job(ctx dbos.DBOSContext, in JobInput) (int, error) {
	return dbos.RunAsStep(ctx, func(context.Context) (int, error) {
		return in.N * in.N, nil
	}, dbos.WithStepName("square"))
}
