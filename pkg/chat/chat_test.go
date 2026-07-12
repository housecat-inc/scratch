package chat

import (
	"context"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
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

func TestAgentName(t *testing.T) {
	a := assert.New(t)

	a.Equal("codex", AgentName(CodexAgent{}))
	a.Equal("echo", AgentName(EchoAgent{}))
}

func TestWithDir(t *testing.T) {
	a := assert.New(t)

	a.Equal(CodexAgent{Dir: "/repo"}, WithDir(CodexAgent{}, "/repo"))
	a.Equal(ClaudeAgent{Dir: "/repo"}, WithDir(ClaudeAgent{}, "/repo"))
	a.Equal(EchoAgent{Delay: time.Second}, WithDir(EchoAgent{Delay: time.Second}, "/repo"))
	a.Equal(CodexAgent{}, WithDir(CodexAgent{}, ""))
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
	asst := view.Messages[1]
	a.Equal(db.MessageStatusError, asst.Status)
	parts := view.Parts[asst.ID]
	r.NotEmpty(parts)
	last := parts[len(parts)-1]
	a.Equal(PartError, last.Kind)
	a.Contains(last.Text, "agent exploded")
}

func TestStopThreadCancelsRunningTurn(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	block := make(chan struct{})
	svc := newTestService(t, blockingAgent{release: block})
	thread, err := svc.CreateThread("", "")
	r.NoError(err)
	_, err = svc.Send(thread.ID, "stop me")
	r.NoError(err)

	r.Eventually(func() bool {
		view, err := svc.View(thread.ID)
		return err == nil && view.Streaming
	}, 5*time.Second, 10*time.Millisecond)

	r.NoError(svc.StopThread(thread.ID))
	view := waitComplete(t, svc, thread.ID)
	asst := view.Messages[1]
	a.Equal(db.MessageStatusError, asst.Status)
	a.Contains(asst.Body, "The turn was stopped.")
	close(block)
}

func TestStopThreadNotBusy(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{Delay: time.Millisecond})
	thread, err := svc.CreateThread("", "")
	r.NoError(err)

	err = svc.StopThread(thread.ID)
	a.True(IsThreadNotBusy(err))
}

func TestRecoverFinishesOrphanedStreaming(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, EchoAgent{Delay: time.Millisecond}, nil)
	t.Cleanup(svc.Close)

	thread, err := svc.CreateThread("", "orphaned")
	r.NoError(err)
	orphan, err := store.AddMessage(db.NewMessage{
		Author:   "agent:echo",
		Role:     db.MessageRoleAssistant,
		Status:   db.MessageStatusStreaming,
		ThreadID: thread.ID,
	})
	r.NoError(err)

	_, err = svc.Send(thread.ID, "hi")
	r.True(IsThreadBusy(err))

	r.NoError(svc.Recover())

	swept, err := store.GetMessage(orphan.ID)
	r.NoError(err)
	a.Equal(db.MessageStatusError, swept.Status)
	a.Contains(swept.Body, "interrupted by a restart")

	_, err = svc.Send(thread.ID, "hi again")
	r.NoError(err)
	waitComplete(t, svc, thread.ID)
}

func TestRecoverLeavesAliveTurns(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, aliveAgent{}, nil)
	t.Cleanup(svc.Close)

	thread, err := svc.CreateThread("", "durable")
	r.NoError(err)
	pending, err := store.AddMessage(db.NewMessage{
		Author:   "agent:alive",
		Role:     db.MessageRoleAssistant,
		Status:   db.MessageStatusStreaming,
		ThreadID: thread.ID,
	})
	r.NoError(err)

	r.NoError(svc.Recover())

	kept, err := store.GetMessage(pending.ID)
	r.NoError(err)
	a.Equal(db.MessageStatusStreaming, kept.Status)
}

func TestResolveElicitationRecordsResultBeforeDeliver(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	store, err := db.New(":memory:")
	r.NoError(err)
	t.Cleanup(func() { store.Close() })
	svc := NewService(store, EchoAgent{}, nil)
	t.Cleanup(svc.Close)
	resolver := &spyResolver{}
	svc.SetResolver(resolver)

	thread, err := svc.CreateThread("", "review")
	r.NoError(err)
	msg, err := store.AddMessage(db.NewMessage{
		Author:   "workflow:test",
		Role:     db.MessageRoleAssistant,
		Status:   db.MessageStatusStreaming,
		ThreadID: thread.ID,
	})
	r.NoError(err)
	_, err = store.AddMessageEvent(msg.ID, EventElicitation, `{"elicitationId":"review-1","message":"Review","topic":"elicit/review","workflowId":"workflow-1"}`)
	r.NoError(err)

	svc.store = eventFailStore{ThreadStore: store}
	err = svc.ResolveElicitation(msg.ID, "review-1", elicit.ActionDecline, nil)
	a.ErrorContains(err, "record result")
	a.False(resolver.delivered)
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

type aliveAgent struct{}

func (aliveAgent) Alive(turn Turn) bool { return true }

func (aliveAgent) Author() string { return "agent:alive" }

func (aliveAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	<-ctx.Done()
	return "", ErrTurnPending
}

type failingAgent struct{}

func (failingAgent) Author() string { return "agent:fail" }

func (failingAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	return "", errors.New("agent exploded")
}

type eventFailStore struct {
	db.ThreadStore
}

func (s eventFailStore) AddMessageEvent(messageID int64, eventType, data string) (db.MessageEvent, error) {
	if eventType == EventElicitationResult {
		return db.MessageEvent{}, errors.New("insert result")
	}
	return s.ThreadStore.AddMessageEvent(messageID, eventType, data)
}

type spyResolver struct {
	delivered bool
}

func (r *spyResolver) Deliver(workflowID, topic, idempotencyKey string, reply elicit.Reply) error {
	r.delivered = true
	return nil
}
