package chat

import (
	"bufio"
	"bytes"
	"context"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
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

	t.Run("reports a recorded send by client id", func(t *testing.T) {
		a := assert.New(t)
		_, h := setup(t)
		post(h, "/codex/threads", url.Values{"client_id": {"c1"}, "prompt": {"hello"}})

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/codex/sends?id=c1&id=missing", nil))

		a.JSONEq(`{"c1":{"status":"sent","threadId":"1","turnId":"1"}}`, rec.Body.String())
	})

	t.Run("deletes a thread and tells the rail", func(t *testing.T) {
		a := assert.New(t)
		svc, h := setup(t)
		post(h, "/codex/threads", url.Values{"client_id": {"c1"}, "prompt": {"bye"}})

		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/codex/threads/1", nil))

		a.Equal(http.StatusNoContent, rec.Code)
		a.Contains(rec.Header().Get("HX-Trigger"), "thread-deleted")
		threads, err := svc.Threads()
		a.NoError(err)
		a.Empty(threads)
	})

	upload := func(t *testing.T, h http.Handler, name, body string) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, err := w.CreateFormFile("file", name)
		require.NoError(t, err)
		_, err = part.Write([]byte(body))
		require.NoError(t, err)
		require.NoError(t, w.Close())
		req := httptest.NewRequest(http.MethodPost, "/codex/attachments", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec
	}

	t.Run("when attaching files", func(t *testing.T) {
		t.Run("uploads a draft before any thread exists", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			svc, h := setup(t)

			rec := upload(t, h, "notes.txt", "hello")

			a.Equal(http.StatusOK, rec.Code)
			a.Contains(rec.Body.String(), `data-attachment-id="1"`)
			a.Contains(rec.Body.String(), "data-attachment-remove")
			attachment, err := svc.Attachment(1)
			r.NoError(err)
			t.Cleanup(func() { os.Remove(attachment.FilePath) })
			a.Zero(attachment.ThreadID)
			a.Nil(attachment.MessageID)
		})

		t.Run("links drafts to the message that starts a thread", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			svc, h := setup(t)
			upload(t, h, "a.txt", "a")
			upload(t, h, "b.txt", "b")
			t.Cleanup(func() {
				for _, id := range []int64{1, 2} {
					if attachment, err := svc.Attachment(id); err == nil {
						os.Remove(attachment.FilePath)
					}
				}
			})

			post(h, "/codex/threads", url.Values{"attachments": {"1,2"}, "client_id": {"c1"}, "prompt": {"look"}})

			r.Eventually(func() bool {
				view, err := svc.View(1)
				return err == nil && !view.Streaming && len(view.Messages) == 2
			}, 5*time.Second, 10*time.Millisecond)
			view, err := svc.View(1)
			r.NoError(err)
			r.Len(view.Attachments[view.Messages[0].ID], 2)
			a.Equal(int64(1), view.Attachments[view.Messages[0].ID][0].ThreadID)

			req := httptest.NewRequest(http.MethodGet, "/codex?chat=1", nil)
			req.Header.Set("HX-Request", "true")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			a.Contains(rec.Body.String(), "user-message-attachments")
			a.Contains(rec.Body.String(), `href="/codex/attachments/2"`)
		})

		t.Run("serves an attachment inline", func(t *testing.T) {
			a := assert.New(t)
			svc, h := setup(t)
			upload(t, h, "notes.txt", "hello")
			t.Cleanup(func() {
				if attachment, err := svc.Attachment(1); err == nil {
					os.Remove(attachment.FilePath)
				}
			})

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/codex/attachments/1", nil))

			a.Equal(http.StatusOK, rec.Code)
			a.Equal("hello", rec.Body.String())
			a.Contains(rec.Header().Get("Content-Disposition"), "inline")
		})

		t.Run("removes a draft", func(t *testing.T) {
			a := assert.New(t)
			svc, h := setup(t)
			upload(t, h, "notes.txt", "hello")

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(http.MethodDelete, "/codex/attachments/1", nil))

			a.Equal(http.StatusNoContent, rec.Code)
			_, err := svc.Attachment(1)
			a.True(db.IsAttachmentNotFound(err))
		})

		t.Run("rejects a send naming a missing attachment", func(t *testing.T) {
			a := assert.New(t)
			_, h := setup(t)

			rec := post(h, "/codex/threads", url.Values{"attachments": {"99"}, "client_id": {"c1"}, "prompt": {"look"}})

			a.Equal(http.StatusBadRequest, rec.Code)
			a.Contains(rec.Body.String(), "no longer available")
		})
	})

	t.Run("streams the thread as server sent events", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		svc, h := setup(t)
		post(h, "/codex/threads", url.Values{"client_id": {"c1"}, "prompt": {"ping"}})
		r.Eventually(func() bool {
			view, err := svc.View(1)
			return err == nil && !view.Streaming && len(view.Messages) == 2
		}, 5*time.Second, 10*time.Millisecond)
		srv := httptest.NewServer(h)
		t.Cleanup(srv.Close)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		t.Cleanup(cancel)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/codex/chat/events?thread=1", nil)
		r.NoError(err)

		res, err := http.DefaultClient.Do(req)
		r.NoError(err)
		t.Cleanup(func() { res.Body.Close() })
		a.Equal("text/event-stream", res.Header.Get("Content-Type"))
		frame, err := bufio.NewReader(res.Body).ReadString('\n')
		r.NoError(err)
		a.True(strings.HasPrefix(frame, "data: "))
	})
}

func TestCodexServerMiddleware(t *testing.T) {
	tests := []struct {
		location string
		method   string
		name     string
		referer  string
		want     string
	}{
		{location: "/inbox/tasks/1", method: http.MethodPost, name: "carries the open chat through a post redirect", referer: "http://example.com/inbox/tasks?chat=3", want: "/inbox/tasks/1?chat=3"},
		{location: "/inbox/tasks/1", method: http.MethodPost, name: "leaves a redirect alone when no chat is open", referer: "http://example.com/inbox/tasks", want: "/inbox/tasks/1"},
		{location: "/inbox/tasks?chat=4", method: http.MethodPost, name: "keeps a chat the redirect already names", referer: "http://example.com/?chat=3", want: "/inbox/tasks?chat=4"},
		{location: "/inbox/tasks/1", method: http.MethodPost, name: "ignores a chat from another site", referer: "http://other.com/?chat=3", want: "/inbox/tasks/1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			store, err := db.New(":memory:")
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			svc := NewService(store, EchoAgent{}, nil)
			t.Cleanup(svc.Close)
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, tt.location, http.StatusSeeOther)
			})
			req := httptest.NewRequest(tt.method, "http://example.com/inbox/tasks", nil)
			req.Header.Set("Referer", tt.referer)
			rec := httptest.NewRecorder()

			NewCodexServer(svc).Middleware(next).ServeHTTP(rec, req)

			a.Equal(tt.want, rec.Header().Get("Location"))
		})
	}
}
