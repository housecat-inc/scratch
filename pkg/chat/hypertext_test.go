package chat

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLegacyChatPagesAreNotMounted(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, EchoAgent{}, nil)
	t.Cleanup(svc.Close)
	srv := NewServer(svc, nil).Handler()

	for _, path := range []string{"/chat", "/chat/1"} {
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		a.Equal(http.StatusNotFound, rec.Code, path)
	}
}

func TestNewPopoutNormalizesProviderModel(t *testing.T) {
	tests := []struct {
		name       string
		provider   string
		wantAnchor string
	}{
		{
			name:       "valid model",
			provider:   "codex:gpt-5",
			wantAnchor: `{"agent":"codex","model":"gpt-5"}`,
		},
		{
			name:       "invalid model",
			provider:   "codex:not-real",
			wantAnchor: `{"agent":"codex"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			store, err := db.New(":memory:")
			r.NoError(err)
			t.Cleanup(func() { store.Close() })
			svc := NewService(store, EchoAgent{}, nil)
			svc.RegisterAgent("codex", EchoAgent{})
			t.Cleanup(svc.Close)
			srv := NewServer(svc, nil).Handler()

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/chat/popout/new?provider_model="+url.QueryEscape(tt.provider), nil)
			srv.ServeHTTP(rec, req)
			a.Equal(http.StatusOK, rec.Code)

			threads, err := svc.Threads()
			r.NoError(err)
			r.Len(threads, 1)
			a.JSONEq(tt.wantAnchor, threads[0].Anchor)
		})
	}
}

func TestFormPropsUnescapesDefaultEntities(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	form := FormProps("/submit", elicit.Prompt{
		ElicitationID: "pr/review",
		Message:       "Review",
		Order:         []string{"title"},
		RequestedSchema: &jsonschema.Schema{
			Properties: map[string]*jsonschema.Schema{
				"title": {
					Default: json.RawMessage(`"Add edit &amp; fork actions"`),
					Type:    "string",
				},
			},
			Type: "object",
		},
	})

	r.Len(form.Fields, 1)
	a.Equal("Add edit & fork actions", form.Fields[0].Value)
}

func TestChatMessageTransport(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, EchoAgent{Delay: time.Millisecond}, nil)
	t.Cleanup(svc.Close)
	srv := NewServer(svc, nil).Handler()

	thread, err := svc.CreateThread("", "")
	r.NoError(err)

	form := url.Values{"prompt": []string{"hi agent"}}
	req := httptest.NewRequest(http.MethodPost, "/chat/"+strconv.FormatInt(thread.ID, 10)+"/messages", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	a.Equal(http.StatusNoContent, rec.Code)

	r.Eventually(func() bool {
		view, err := svc.View(thread.ID)
		return err == nil && !view.Streaming && len(view.Messages) == 2
	}, 5*time.Second, 10*time.Millisecond)

	view, err := svc.View(thread.ID)
	r.NoError(err)
	a.Equal("hi agent", view.Messages[0].Body)
	a.Equal("You said: hi agent", view.Messages[1].Body)
}

func TestChatMessageTransportQueuesWhileStreaming(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	block := make(chan struct{})
	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, blockingAgent{release: block}, nil)
	t.Cleanup(svc.Close)
	srv := NewServer(svc, nil).Handler()

	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	_, err = svc.Send(thread.ID, "first")
	r.NoError(err)

	form := url.Values{"prompt": []string{"second"}}
	req := httptest.NewRequest(http.MethodPost, "/chat/"+strconv.FormatInt(thread.ID, 10)+"/messages", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	a.Equal(http.StatusNoContent, rec.Code)

	close(block)
	r.Eventually(func() bool {
		view, err := svc.View(thread.ID)
		return err == nil && !view.Streaming && len(view.Messages) == 4
	}, 5*time.Second, 10*time.Millisecond)

	view, err := svc.View(thread.ID)
	r.NoError(err)
	a.Equal("second", view.Messages[2].Body)
	a.Equal(db.MessageStatusComplete, view.Messages[3].Status)
}

func TestChatStopTransport(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	block := make(chan struct{})
	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, blockingAgent{release: block}, nil)
	t.Cleanup(svc.Close)
	srv := NewServer(svc, nil).Handler()

	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	_, err = svc.Send(thread.ID, "stop")
	r.NoError(err)

	req := httptest.NewRequest(http.MethodPost, "/chat/"+strconv.FormatInt(thread.ID, 10)+"/stop", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	a.Equal(http.StatusNoContent, rec.Code)

	close(block)
	r.Eventually(func() bool {
		view, err := svc.View(thread.ID)
		return err == nil && !view.Streaming && len(view.Messages) == 2 && view.Messages[1].Status == db.MessageStatusError
	}, 5*time.Second, 10*time.Millisecond)
}
