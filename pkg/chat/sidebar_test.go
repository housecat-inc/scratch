package chat

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSidebarChats(t *testing.T) {
	for _, count := range []int{0, 2, 6} {
		t.Run(strconv.Itoa(count), func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			store, err := db.New(":memory:")
			r.NoError(err)
			t.Cleanup(func() { store.Close() })
			svc := NewService(store, EchoAgent{}, nil)
			t.Cleanup(svc.Close)
			current, err := svc.CreateThread("", "Current")
			r.NoError(err)
			active, err := svc.CreateThread("", "Still working")
			r.NoError(err)
			message, err := store.AddMessage(db.NewMessage{Role: db.MessageRoleAssistant, Status: db.MessageStatusStreaming, ThreadID: active.ID})
			r.NoError(err)
			for i := 0; i < count; i++ {
				_, err = svc.CreateThread("", "Recent "+strconv.Itoa(i))
				r.NoError(err)
			}
			_, err = store.AddThread(db.ThreadKindChat, "Scheduled example", `{"agent":"workflow:schedule:example"}`)
			r.NoError(err)
			server := NewServer(svc, nil)
			props, err := server.sidebarProps(current.ID)
			r.NoError(err)
			r.Len(props.Running, 1)
			a.Equal(active.ID, props.Running[0].ID)
			r.Len(props.Recent, min(count, 3))
			for i, item := range props.Recent {
				a.Equal("Recent "+strconv.Itoa(count-i-1), item.Title)
			}
			response := httptest.NewRecorder()
			server.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/chat/"+strconv.FormatInt(current.ID, 10)+"/sidebar", nil))
			a.Equal(http.StatusOK, response.Code)
			a.Contains(response.Body.String(), "Agent running")
			a.NotContains(response.Body.String(), ">Current<")
			a.NotContains(response.Body.String(), "Scheduled example")
			_, err = store.FinishMessage(message.ID, db.MessageStatusComplete)
			r.NoError(err)
			props, err = server.sidebarProps(current.ID)
			r.NoError(err)
			a.Empty(props.Running)
		})
	}
}
