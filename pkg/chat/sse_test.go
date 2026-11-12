package chat

import (
	"net/http/httptest"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteEventData(t *testing.T) {
	t.Run("strips carriage returns so every line stays valid SSE", func(t *testing.T) {
		a := assert.New(t)
		rec := httptest.NewRecorder()

		writeEventData(rec, "line one\r\nline two\rline three")

		a.Equal("data: line one\ndata: line two\ndata: line three\n\n", rec.Body.String())
	})
}

func TestView(t *testing.T) {
	t.Run("shows the first unresolved form until every form is answered", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		store, err := db.New(":memory:")
		r.NoError(err)
		t.Cleanup(func() { store.Close() })
		svc := NewService(store, EchoAgent{}, nil)
		t.Cleanup(svc.Close)

		thread, err := svc.CreateThread("", "")
		r.NoError(err)
		asst, err := store.AddMessage(db.NewMessage{
			Author:   "workflow:contact",
			Role:     db.MessageRoleAssistant,
			Status:   db.MessageStatusStreaming,
			ThreadID: thread.ID,
		})
		r.NoError(err)

		_, err = store.AddMessageEvent(asst.ID, EventElicitation, `{"elicitationId":"wf/first","message":"first"}`)
		r.NoError(err)
		_, err = store.AddMessageEvent(asst.ID, EventElicitation, `{"elicitationId":"wf/second","message":"second"}`)
		r.NoError(err)

		view, err := svc.View(thread.ID)
		r.NoError(err)
		a.Equal("wf/first", view.Forms[asst.ID].ElicitationID)

		_, err = store.AddMessageEvent(asst.ID, EventElicitationResult, `{"action":"accept","elicitationId":"wf/first"}`)
		r.NoError(err)

		view, err = svc.View(thread.ID)
		r.NoError(err)
		a.Equal("wf/second", view.Forms[asst.ID].ElicitationID)

		_, err = store.AddMessageEvent(asst.ID, EventElicitationResult, `{"action":"decline","elicitationId":"wf/second"}`)
		r.NoError(err)

		view, err = svc.View(thread.ID)
		r.NoError(err)
		_, pending := view.Forms[asst.ID]
		a.False(pending)
		a.Equal(map[int64]elicit.Prompt{}, view.Forms)
	})
}
