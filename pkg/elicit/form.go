package elicit

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

func Form[T any](ctx dbos.DBOSContext, p Prompter, key, message string, defaults T, timeout time.Duration) (T, string, error) {
	var out T
	schema, err := SchemaFor(defaults)
	if err != nil {
		return out, "", err
	}
	workflowID, err := dbos.GetWorkflowID(ctx)
	if err != nil {
		return out, "", errors.Wrap(err, "get workflow id")
	}
	prompt := Prompt{
		ElicitationID:   workflowID + "/" + key,
		Message:         message,
		Order:           schema.PropertyOrder,
		RequestedSchema: schema,
		Topic:           "elicit/" + key,
		WorkflowID:      workflowID,
	}
	if _, err := dbos.RunAsStep(ctx, func(context.Context) (string, error) {
		return "", p.Prompt(prompt)
	}, dbos.WithStepName("elicit/"+key)); err != nil {
		return out, "", errors.Wrap(err, "prompt")
	}

	reply, err := dbos.Recv[Reply](ctx, prompt.Topic, timeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return out, ActionCancel, nil
	}
	if errors.Is(err, context.Canceled) {
		blockForRecovery()
	}
	if err != nil {
		return out, "", errors.Wrap(err, "receive reply")
	}
	if reply.Action != ActionAccept {
		return out, reply.Action, nil
	}
	if err := json.Unmarshal([]byte(reply.Content), &out); err != nil {
		return out, "", errors.Wrap(err, "decode reply content")
	}
	return out, ActionAccept, nil
}

// Returning a cancellation error would record a terminal ERROR outcome, which
// resumeWorkflows excludes; parking the goroutine keeps the workflow PENDING so
// the next launch recovers it back to this Recv.
func blockForRecovery() {
	select {}
}
