package chat

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T, agent Agent) *Service {
	t.Helper()
	store, err := db.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, agent, nil)
	t.Cleanup(svc.Close)
	return svc
}

func waitComplete(t *testing.T, svc *Service, threadID int64) ThreadView {
	t.Helper()
	var view ThreadView
	require.Eventually(t, func() bool {
		v, err := svc.View(threadID)
		if err != nil {
			return false
		}
		view = v
		return !v.Streaming && len(v.Messages) > 0
	}, 5*time.Second, 10*time.Millisecond)
	return view
}

func TestSendEcho(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{Delay: time.Millisecond})

	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	a.Equal(db.ThreadKindChat, thread.Kind)

	user, err := svc.Send(thread.ID, "hello there")
	r.NoError(err)
	a.Equal(db.MessageRoleUser, user.Role)
	a.Equal("hello there", user.Body)

	view := waitComplete(t, svc, thread.ID)
	r.Len(view.Messages, 2)

	asst := view.Messages[1]
	a.Equal(db.MessageRoleAssistant, asst.Role)
	a.Equal("agent:echo", asst.Author)
	a.Equal(db.MessageStatusComplete, asst.Status)
	a.Equal("You said: hello there", asst.Body)
	r.NotNil(asst.ParentID)
	a.Equal(user.ID, *asst.ParentID)
	r.NotNil(asst.CompletedAt)

	got, err := svc.Thread(thread.ID)
	r.NoError(err)
	a.Equal("hello there", got.Title)
}

func TestSendBusy(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	block := make(chan struct{})
	svc := newTestService(t, blockingAgent{release: block})

	thread, err := svc.CreateThread("", "busy")
	r.NoError(err)

	_, err = svc.Send(thread.ID, "first")
	r.NoError(err)

	_, err = svc.Send(thread.ID, "second")
	a.True(IsThreadBusy(err))

	close(block)
	view := waitComplete(t, svc, thread.ID)
	a.Equal(db.MessageStatusComplete, view.Messages[1].Status)

	_, err = svc.Send(thread.ID, "third")
	a.NoError(err)
}

func TestSendAgentError(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, failingAgent{})

	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	_, err = svc.Send(thread.ID, "boom")
	r.NoError(err)

	view := waitComplete(t, svc, thread.ID)
	r.Len(view.Messages, 2)
	a.Equal(db.MessageStatusError, view.Messages[1].Status)
	a.Contains(view.Messages[1].Body, "agent exploded")
}

func TestSendMissingThread(t *testing.T) {
	a := assert.New(t)

	svc := newTestService(t, EchoAgent{})
	_, err := svc.Send(404, "hi")
	a.True(db.IsThreadNotFound(err))
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		in   string
		n    int
		name string
		want string
	}{
		{in: "short", n: 10, name: "short", want: "short"},
		{in: "  padded  ", n: 10, name: "trims", want: "padded"},
		{in: "exactly-10", n: 10, name: "boundary", want: "exactly-10"},
		{in: "a very long prompt indeed", n: 10, name: "long", want: "a very lon…"},
		{in: "héllo wörld", n: 5, name: "unicode", want: "héllo…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.New(t).Equal(tc.want, truncate(tc.in, tc.n))
		})
	}
}

type blockingAgent struct {
	release chan struct{}
}

func (blockingAgent) Author() string { return "agent:block" }

func (a blockingAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case <-a.release:
	}
	emit(DeltaEvent("done"))
	return "", nil
}

type failingAgent struct{}

func (failingAgent) Author() string { return "agent:fail" }

func (failingAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	return "", errors.New("agent exploded")
}
