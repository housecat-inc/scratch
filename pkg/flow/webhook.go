package flow

import (
	"context"
	"fmt"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
)

const (
	defaultWebhookTimeout = time.Hour
	webhookTopic          = "webhook"
)

type WebhookInput struct{}

type WebhookPayload struct {
	Event   string `json:"event"`
	Message string `json:"message"`
}

func (e *Engine) webhook(ctx dbos.DBOSContext, _ WebhookInput) (string, error) {
	id, err := dbos.GetWorkflowID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "workflow id")
	}
	if _, err := act(ctx, "respond/webhook", "Waiting for webhook", "", func(context.Context) (string, error) {
		return webhookInstructions(id), nil
	}); err != nil {
		return "", errors.Wrap(err, "await webhook")
	}
	payload, err := dbos.Recv[WebhookPayload](ctx, webhookTopic, e.webhookTimeout)
	if errors.Is(err, context.DeadlineExceeded) {
		return "No webhook received", nil
	}
	if err != nil {
		return "", errors.Wrap(err, "receive webhook")
	}
	if _, err := act(ctx, "respond/received", "Webhook received", payload.Event, func(context.Context) (string, error) {
		return fmt.Sprintf("**%s**\n\n%s", payload.Event, payload.Message), nil
	}); err != nil {
		return "", errors.Wrap(err, "record webhook")
	}
	return "Received " + payload.Event, nil
}

func (e *Engine) DeliverWebhook(workflowID string, payload WebhookPayload, idempotencyKey string) error {
	if _, _, err := e.workflow(workflowID); err != nil {
		return err
	}
	var opts []dbos.SendOption
	if idempotencyKey != "" {
		opts = append(opts, dbos.WithIdempotencyKey(idempotencyKey))
	}
	return errors.Wrap(dbos.Send(e.ctx, workflowID, payload, webhookTopic, opts...), "deliver webhook")
}

func webhookInstructions(id string) string {
	return "Waiting for an external callback. Deliver a payload with:\n\n```\n" + webhookCurl(id) + "\n```"
}

func webhookCurl(id string) string {
	return fmt.Sprintf("curl -X POST localhost:8888/webhooks/%s \\\n"+
		"  -H 'Idempotency-Key: abc123' \\\n"+
		"  -d '{\"event\":\"deploy.finished\",\"message\":\"build 42 is live\"}'", id)
}
