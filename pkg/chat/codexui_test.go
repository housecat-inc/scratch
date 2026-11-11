package chat

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexServer(t *testing.T) {
	post := func(h http.Handler, path string, form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}
	setup := func(t *testing.T) (*Service, http.Handler) {
		store, err := db.New(":memory:")
		require.NoError(t, err)
		t.Cleanup(func() { store.Close() })
		svc := NewService(store, EchoAgent{Delay: time.Millisecond}, nil)
		t.Cleanup(svc.Close)
		return svc, NewCodexServer(svc).WithLabel("todo").Handler()
	}

	t.Run("starts a labeled thread from the first message", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		svc, h := setup(t)

		rec := post(h, "/codex/threads?surface=mail", url.Values{"client_id": {"c1"}, "optimistic": {"true"}, "prompt": {"hello"}})

		a.Equal(http.StatusOK, rec.Code)
		a.Contains(rec.Body.String(), `data-thread-id="1"`)
		a.Contains(rec.Body.String(), `data-outgoing-message="c1"`)
		thread, err := svc.Thread(1)
		r.NoError(err)
		a.Equal("todo", svc.ThreadLabel(thread))
		a.Equal("hello", thread.Title)
	})

	t.Run("answers a retried send without sending twice", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		svc, h := setup(t)
		form := url.Values{"client_id": {"c1"}, "optimistic": {"true"}, "prompt": {"hello"}}

		first := post(h, "/codex/threads", form)
		second := post(h, "/codex/threads", form)

		a.Equal(first.Body.String(), second.Body.String())
		threads, err := svc.Threads()
		r.NoError(err)
		a.Len(threads, 1)
	})

	t.Run("sends a follow-up to the thread its first message started", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		svc, h := setup(t)
		post(h, "/codex/threads", url.Values{"client_id": {"c1"}, "prompt": {"first"}})

		rec := post(h, "/codex/threads", url.Values{"client_id": {"c2"}, "client_thread_id": {"c1"}, "enqueue": {"true"}, "prompt": {"second"}})

		a.Contains(rec.Body.String(), `data-thread-id="1"`)
		r.Eventually(func() bool {
			view, err := svc.View(1)
			return err == nil && !view.Streaming && len(view.Messages) == 4
		}, 5*time.Second, 10*time.Millisecond)
	})

	t.Run("rejects a follow-up before its thread exists", func(t *testing.T) {
		a := assert.New(t)
		_, h := setup(t)

		rec := post(h, "/codex/threads", url.Values{"client_id": {"c2"}, "client_thread_id": {"c1"}, "prompt": {"second"}})

		a.Equal(http.StatusConflict, rec.Code)
		a.Contains(rec.Body.String(), "data-chat-send-error")
	})

	t.Run("renders the thread's turns for the rail", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		svc, h := setup(t)
		post(h, "/codex/threads", url.Values{"client_id": {"c1"}, "prompt": {"ping"}})
		r.Eventually(func() bool {
			view, err := svc.View(1)
			return err == nil && !view.Streaming && len(view.Messages) == 2
		}, 5*time.Second, 10*time.Millisecond)

		req := httptest.NewRequest(http.MethodGet, "/codex?surface=mail&chat=1", nil)
		req.Header.Set("HX-Request", "true")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)

		a.Equal("false", rec.Header().Get("HX-Push-Url"))
		a.Contains(rec.Body.String(), `id="mail-chat-rail-content"`)
		a.Contains(rec.Body.String(), `data-turn-id="1/1"`)
		a.Contains(rec.Body.String(), "You said: ping")
	})
}
