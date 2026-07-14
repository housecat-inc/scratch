package flow

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestWebhookWorkflow(t *testing.T) {
	r := require.New(t)
	e := newEngine(t)

	r.NoError(e.Start("webhook", "wh-1"))
	r.Eventually(func() bool {
		run, err := e.Run("wh-1")
		return err == nil && run.Running() && len(run.Steps) > 0
	}, 5*time.Second, 20*time.Millisecond)

	r.NoError(e.DeliverWebhook("wh-1", WebhookPayload{Event: "deploy.finished", Message: "build 42 is live"}, "key-1"))
	e.Await("wh-1", 5*time.Second)

	run, err := e.Run("wh-1")
	r.NoError(err)
	r.True(run.Done())
	r.Equal("Received deploy.finished", run.Result)
	r.Equal("Webhook received", run.Steps[len(run.Steps)-1].Title)
	r.Contains(run.Steps[len(run.Steps)-1].Detail, "build 42 is live")
}

func TestWebhookIdempotentDelivery(t *testing.T) {
	r := require.New(t)
	e := newEngine(t)

	r.NoError(e.Start("webhook", "wh-2"))
	r.Eventually(func() bool {
		run, err := e.Run("wh-2")
		return err == nil && run.Running() && len(run.Steps) > 0
	}, 5*time.Second, 20*time.Millisecond)

	payload := WebhookPayload{Event: "push", Message: "sha abc"}
	r.NoError(e.DeliverWebhook("wh-2", payload, "dup-key"))
	r.NoError(e.DeliverWebhook("wh-2", payload, "dup-key"))
	e.Await("wh-2", 5*time.Second)

	run, err := e.Run("wh-2")
	r.NoError(err)
	r.True(run.Done())
	r.Equal("Received push", run.Result)
}

func TestWebhookUnknownWorkflow(t *testing.T) {
	r := require.New(t)
	e := newEngine(t)
	err := e.DeliverWebhook("nope", WebhookPayload{Event: "x"}, "")
	r.ErrorIs(err, ErrRunNotFound)
}
