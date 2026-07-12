package chat

import (
	"log/slog"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteMessagesEventStripsCarriageReturns(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, EchoAgent{}, nil)
	t.Cleanup(svc.Close)

	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	_, err = store.AddMessage(db.NewMessage{
		Author:   "you",
		Body:     "line one\r\nline two\rline three",
		Role:     db.MessageRoleUser,
		ThreadID: thread.ID,
	})
	r.NoError(err)

	srv := NewServer(svc, slog.Default())
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/chat/1/events", nil)
	r.NoError(srv.writeMessagesEvent(rec, req, thread.ID))

	body := rec.Body.String()
	a.NotContains(body, "\r")
	a.Contains(body, "line one")
	for line := range strings.Lines(body) {
		trimmed := strings.TrimRight(line, "\n")
		if trimmed == "" || strings.HasPrefix(trimmed, "event: ") {
			continue
		}
		a.True(strings.HasPrefix(trimmed, "data: "), "unexpected SSE line %q", trimmed)
	}
}

func TestViewPicksFirstUnresolvedForm(t *testing.T) {
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
}
