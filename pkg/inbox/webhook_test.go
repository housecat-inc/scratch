package inbox

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/require"
)

func TestWebhook(t *testing.T) {
	t.Run("delivers the payload to the waiting workflow", func(t *testing.T) {
		r := require.New(t)
		log := slog.New(slog.NewTextHandler(io.Discard, nil))
		srv, chatSvc, flows := webhookServer(t, flow.Deps{
			Drafter: stubDrafter{},
			Greeter: flow.HeuristicGreeter(),
			Log:     log,
			Updater: stubUpdater{},
		}, chat.EchoAgent{Delay: 10 * time.Millisecond})

		thread, workflowID, err := chatSvc.CreateWorkflowThread("webhook", "Webhook")
		r.NoError(err)
		r.NoError(flows.Start("webhook", workflowID))

		page := "/inbox/workflows/" + strconv.FormatInt(thread.ID, 10)
		r.Eventually(func() bool {
			return strings.Contains(get(t, srv, page), "Waiting for an external callback")
		}, 5*time.Second, 25*time.Millisecond)

		waiting := get(t, srv, page)
		r.Contains(waiting, "mail-copy-btn")
		r.Contains(waiting, "/webhooks/"+workflowID)

		req, err := http.NewRequest(http.MethodPost, srv.URL+"/webhooks/"+workflowID,
			strings.NewReader(`{"event":"deploy.finished","message":"build 42 is live"}`))
		r.NoError(err)
		req.Header.Set("Idempotency-Key", "abc123")
		resp, err := srv.Client().Do(req)
		r.NoError(err)
		resp.Body.Close()
		r.Equal(http.StatusAccepted, resp.StatusCode)

		r.Eventually(func() bool {
			body := get(t, srv, page)
			return strings.Contains(body, "Webhook received") && strings.Contains(body, "build 42 is live")
		}, 5*time.Second, 25*time.Millisecond)
	})

	t.Run("rejects an oversized body", func(t *testing.T) {
		r := require.New(t)
		srv, _, _ := webhookServer(t, flow.Deps{Greeter: flow.HeuristicGreeter(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, chat.EchoAgent{})

		body := `{"event":"` + strings.Repeat("a", 128<<10) + `"}`
		resp, err := srv.Client().Post(srv.URL+"/webhooks/whatever", "application/json", strings.NewReader(body))
		r.NoError(err)
		resp.Body.Close()
		r.Equal(http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("returns not found for an unknown workflow", func(t *testing.T) {
		r := require.New(t)
		srv, _, _ := webhookServer(t, flow.Deps{Greeter: flow.HeuristicGreeter(), Log: slog.New(slog.NewTextHandler(io.Discard, nil))}, chat.EchoAgent{})

		resp, err := srv.Client().Post(srv.URL+"/webhooks/nope", "application/json", strings.NewReader(`{"event":"x"}`))
		r.NoError(err)
		resp.Body.Close()
		r.Equal(http.StatusNotFound, resp.StatusCode)
	})
}

func webhookServer(t *testing.T, deps flow.Deps, agent chat.EchoAgent) (*httptest.Server, *chat.Service, *flow.Engine) {
	t.Helper()
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	workflows, err := workflow.New(t.TempDir() + "/wf.db")
	r.NoError(err)
	deps.DBOS = workflows.Ctx()
	flows := flow.New(deps)
	r.NoError(workflows.Launch())
	t.Cleanup(func() { workflows.Close() })

	chatSvc := chat.NewService(store, agent, deps.Log)
	t.Cleanup(chatSvc.Close)

	srv := httptest.NewServer(NewServer(todo.NewService(store), chatSvc, flows, deps.Log).Handler())
	t.Cleanup(srv.Close)
	return srv, chatSvc, flows
}

func get(t *testing.T, srv *httptest.Server, path string) string {
	t.Helper()
	resp, err := srv.Client().Get(srv.URL + path)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return string(body)
}
