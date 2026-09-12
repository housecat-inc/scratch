package inbox

import (
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/require"
)

type summaryContactInput struct {
	Prompt string
}

func summaryContactRun(ctx dbos.DBOSContext, input summaryContactInput) (string, error) {
	return "Accepted", nil
}

func TestContactRunTable(t *testing.T) {
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "scratch.db")
	store, err := db.New(path)
	r.NoError(err)
	defer store.Close()
	flows, err := workflow.New(path)
	r.NoError(err)
	dbos.RegisterWorkflow(flows.Ctx(), summaryContactRun, dbos.WithWorkflowName("contact-intake"))
	r.NoError(flows.Launch())
	defer flows.Close()
	chats := chat.NewService(store, chat.EchoAgent{}, nil)
	defer chats.Close()
	server := NewServer(todo.NewService(store), chats, nil)
	server.ConfigureRunHistory(flows)
	chats.RegisterAgent("contact", chat.EchoAgent{})
	var urls []string
	for _, prompt := range []string{"First contact", "Second contact"} {
		thread, err := chats.CreateThread("contact", prompt)
		r.NoError(err)
		message, err := store.AddMessage(db.NewMessage{Author: "workflow:contact", Role: db.MessageRoleAssistant, ThreadID: thread.ID})
		r.NoError(err)
		handle, err := dbos.RunWorkflow(flows.Ctx(), summaryContactRun, summaryContactInput{Prompt: prompt}, dbos.WithWorkflowID("chat-message-"+strconv.FormatInt(message.ID, 10)))
		r.NoError(err)
		_, err = handle.GetResult()
		r.NoError(err)
		urls = append(urls, "/inbox/workflows/"+strconv.FormatInt(thread.ID, 10))
	}
	response := httptest.NewRecorder()
	server.Handler().ServeHTTP(response, httptest.NewRequest("GET", urls[0], nil))
	r.Equal(200, response.Code)
	r.Contains(response.Body.String(), "<td>First contact</td>")
	r.Contains(response.Body.String(), "<td>Accepted</td>")
	r.NotContains(response.Body.String(), "<td>Second contact</td>")
}
