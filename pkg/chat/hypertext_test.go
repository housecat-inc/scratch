package chat

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/db"
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
