package inbox

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/stretchr/testify/require"
)

func TestWorkflowEventsRejectsNonWorkflowThread(t *testing.T) {
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	chatSvc := chat.NewService(store, chat.EchoAgent{}, nil)
	t.Cleanup(chatSvc.Close)
	thread, err := chatSvc.CreateThreadWithModel("echo", "", "Chat")
	r.NoError(err)

	srv := NewServer(todo.NewService(store), chatSvc, nil, nil).Handler()
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, fmt.Sprintf("/inbox/workflows/%d/events", thread.ID), nil))
	r.Equal(http.StatusNotFound, rec.Code)
}
