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

func TestWebhookRouteDeliversToWorkflow(t *testing.T) {
	r := require.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	workflows, err := workflow.New(t.TempDir() + "/wf.db")
	r.NoError(err)
	flows := flow.New(flow.Deps{
		DBOS:    workflows.Ctx(),
		Drafter: stubDrafter{},
		Greeter: flow.HeuristicGreeter(),
		Log:     log,
		Updater: stubUpdater{},
	})
	r.NoError(workflows.Launch())
	t.Cleanup(func() { workflows.Close() })

	chatSvc := chat.NewService(store, chat.EchoAgent{Delay: 10 * time.Millisecond}, log)
	t.Cleanup(chatSvc.Close)

	srv := httptest.NewServer(NewServer(todo.NewService(store), chatSvc, flows, log).Handler())
	t.Cleanup(srv.Close)

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
}

func TestWebhookRouteUnknownWorkflowIs404(t *testing.T) {
	r := require.New(t)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	workflows, err := workflow.New(t.TempDir() + "/wf.db")
	r.NoError(err)
	flows := flow.New(flow.Deps{DBOS: workflows.Ctx(), Greeter: flow.HeuristicGreeter(), Log: log})
	r.NoError(workflows.Launch())
	t.Cleanup(func() { workflows.Close() })
	chatSvc := chat.NewService(store, chat.EchoAgent{}, log)
	t.Cleanup(chatSvc.Close)

	srv := httptest.NewServer(NewServer(todo.NewService(store), chatSvc, flows, log).Handler())
	t.Cleanup(srv.Close)

	resp, err := srv.Client().Post(srv.URL+"/webhooks/nope", "application/json", strings.NewReader(`{"event":"x"}`))
	r.NoError(err)
	resp.Body.Close()
	r.Equal(http.StatusNotFound, resp.StatusCode)
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
