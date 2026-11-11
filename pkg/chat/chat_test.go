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

type anchorAgent struct{}

func (anchorAgent) Author() string { return "agent:anchor" }

func (anchorAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	emit(Event{Data: `{"agent":"anchor","thread_id":"event"}`, Type: EventAnchor})
	return `{"agent":"anchor","thread_id":"done"}`, nil
}

func TestChat(t *testing.T) {
	t.Run("Send", func(t *testing.T) {
		t.Run("stores the prompt, streams the reply and titles the thread", func(t *testing.T) {
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

			prompt, err := svc.ThreadPrompt(thread.ID)
			r.NoError(err)
			a.Equal("hello there", prompt)
		})

		t.Run("drops replayed duplicate events", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			svc := newTestService(t, duplicateEventAgent{})
			thread, err := svc.CreateThread("", "")
			r.NoError(err)

			_, err = svc.Send(thread.ID, "hello")
			r.NoError(err)

			view := waitComplete(t, svc, thread.ID)
			r.Len(view.Messages, 2)
			asst := view.Messages[1]
			a.Equal("helloagain", asst.Body)

			events, err := svc.store.ListMessageEvents(asst.ID, 0)
			r.NoError(err)
			r.Len(events, 4)
			a.Equal(EventDelta, events[0].Type)
			a.Equal(EventToolCall, events[1].Type)
			a.Equal(EventToolCallUpdate, events[2].Type)
			a.Equal(EventDelta, events[3].Type)
		})

		t.Run("keeps the thread label when the agent updates the anchor", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			svc := newTestService(t, anchorAgent{})

			thread, err := svc.CreateThreadWithLabel("", "", "todo", "")
			r.NoError(err)
			_, err = svc.Send(thread.ID, "hello there")
			r.NoError(err)

			waitComplete(t, svc, thread.ID)

			got, err := svc.Thread(thread.ID)
			r.NoError(err)
			a.Equal("todo", svc.ThreadLabel(got))
			a.JSONEq(`{"agent":"anchor","label":"todo","thread_id":"done"}`, got.Anchor)
		})

		t.Run("queues the next turn while the agent is busy", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			block := make(chan struct{})
			svc := newTestService(t, blockingAgent{release: block})

			thread, err := svc.CreateThread("", "busy")
			r.NoError(err)

			_, err = svc.Send(thread.ID, "first")
			r.NoError(err)

			user, err := svc.Send(thread.ID, "second")
			r.NoError(err)
			a.Equal("second", user.Body)

			close(block)
			view := waitComplete(t, svc, thread.ID)
			r.Len(view.Messages, 4)
			a.Equal("first", view.Messages[0].Body)
			a.Equal(db.MessageStatusComplete, view.Messages[1].Status)
			a.Equal("second", view.Messages[2].Body)
			a.Equal(db.MessageStatusComplete, view.Messages[3].Status)
			r.NotNil(view.Messages[3].ParentID)
			a.Equal(view.Messages[2].ID, *view.Messages[3].ParentID)

			_, err = svc.Send(thread.ID, "third")
			a.NoError(err)
		})

		t.Run("marks the reply as an error when the agent fails", func(t *testing.T) {
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
		})

		t.Run("rejects a missing thread", func(t *testing.T) {
			a := assert.New(t)

			svc := newTestService(t, EchoAgent{})
			_, err := svc.Send(404, "hi")
			a.True(db.IsThreadNotFound(err))
		})
	})

	t.Run("StopThread", func(t *testing.T) {
		t.Run("cancels the running turn", func(t *testing.T) {
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
		})

		t.Run("starts the queued turn after stopping", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			block := make(chan struct{})
			svc := newTestService(t, blockingAgent{release: block})
			thread, err := svc.CreateThread("", "")
			r.NoError(err)
			_, err = svc.Send(thread.ID, "stop me")
			r.NoError(err)
			_, err = svc.Send(thread.ID, "next")
			r.NoError(err)

			r.Eventually(func() bool {
				view, err := svc.View(thread.ID)
				return err == nil && view.Streaming
			}, 5*time.Second, 10*time.Millisecond)

			r.NoError(svc.StopThread(thread.ID))
			close(block)
			view := waitComplete(t, svc, thread.ID)
			r.Len(view.Messages, 4)
			a.Equal(db.MessageStatusError, view.Messages[1].Status)
			a.Contains(view.Messages[1].Body, "The turn was stopped.")
			a.Equal("next", view.Messages[2].Body)
			a.Equal(db.MessageStatusComplete, view.Messages[3].Status)
		})

		t.Run("reports not busy when no turn is running", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			svc := newTestService(t, EchoAgent{Delay: time.Millisecond})
			thread, err := svc.CreateThread("", "")
			r.NoError(err)

			err = svc.StopThread(thread.ID)
			a.True(IsThreadNotBusy(err))
		})
	})

	t.Run("Recover", func(t *testing.T) {
		t.Run("fails orphaned streaming replies and runs the queued prompt", func(t *testing.T) {
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

			queued, err := svc.Send(thread.ID, "hi")
			r.NoError(err)
			a.Equal("hi", queued.Body)

			r.NoError(svc.Recover())

			swept, err := store.GetMessage(orphan.ID)
			r.NoError(err)
			a.Equal(db.MessageStatusError, swept.Status)
			a.Contains(swept.Body, "interrupted by a restart")
			view := waitComplete(t, svc, thread.ID)
			a.Equal("You said: hi", view.Messages[len(view.Messages)-1].Body)

			_, err = svc.Send(thread.ID, "hi again")
			r.NoError(err)
			waitComplete(t, svc, thread.ID)
		})

		t.Run("leaves turns whose agent is still alive", func(t *testing.T) {
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
		})
	})

	t.Run("ResolveElicitation", func(t *testing.T) {
		t.Run("does not deliver when recording the result fails", func(t *testing.T) {
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
		})
	})

	t.Run("ThreadPrompt", func(t *testing.T) {
		t.Run("is empty for a thread without messages", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			svc := newTestService(t, EchoAgent{Delay: time.Millisecond})
			thread, err := svc.CreateThread("", "empty")
			r.NoError(err)

			prompt, err := svc.ThreadPrompt(thread.ID)
			r.NoError(err)
			a.Empty(prompt)
		})
	})

	t.Run("ThreadsByLabel", func(t *testing.T) {
		t.Run("returns only threads with the label", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			svc := newTestService(t, EchoAgent{Delay: time.Millisecond})
			todo, err := svc.CreateThreadWithLabel("", "", "todo", "todo chat")
			r.NoError(err)
			_, err = svc.CreateThreadWithLabel("", "", "scratch", "scratch chat")
			r.NoError(err)
			_, err = svc.CreateThread("", "unlabeled chat")
			r.NoError(err)

			threads, err := svc.ThreadsByLabel("todo")
			r.NoError(err)
			r.Len(threads, 1)
			a.Equal(todo.ID, threads[0].ID)
			a.Equal("todo", svc.ThreadLabel(threads[0]))
		})
	})

	t.Run("AgentName", func(t *testing.T) {
		t.Run("names agents by their type", func(t *testing.T) {
			a := assert.New(t)

			a.Equal("codex", AgentName(CodexAgent{}))
			a.Equal("echo", AgentName(EchoAgent{}))
		})
	})

	t.Run("WithDir", func(t *testing.T) {
		tests := []struct {
			agent Agent
			dir   string
			name  string
			want  Agent
		}{
			{agent: CodexAgent{}, dir: "/repo", name: "sets the codex directory", want: CodexAgent{Dir: "/repo"}},
			{agent: ClaudeAgent{}, dir: "/repo", name: "sets the claude directory", want: ClaudeAgent{Dir: "/repo"}},
			{agent: EchoAgent{Delay: time.Second}, dir: "/repo", name: "leaves agents without a directory unchanged", want: EchoAgent{Delay: time.Second}},
			{agent: CodexAgent{}, dir: "", name: "ignores an empty directory", want: CodexAgent{}},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)

				a.Equal(tt.want, WithDir(tt.agent, tt.dir))
			})
		}
	})

	t.Run("truncate", func(t *testing.T) {
		tests := []struct {
			in   string
			n    int
			name string
			want string
		}{
			{in: "short", n: 10, name: "keeps text under the limit", want: "short"},
			{in: "  padded  ", n: 10, name: "trims surrounding whitespace", want: "padded"},
			{in: "exactly-10", n: 10, name: "keeps text at the limit", want: "exactly-10"},
			{in: "a very long prompt indeed", n: 10, name: "cuts long text with an ellipsis", want: "a very lon…"},
			{in: "héllo wörld", n: 5, name: "counts runes rather than bytes", want: "héllo…"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)

				a.Equal(tt.want, truncate(tt.in, tt.n))
			})
		}
	})
}

type blockingAgent struct {
	release chan struct{}
}

type duplicateEventAgent struct{}

func (duplicateEventAgent) Author() string { return "agent:duplicate" }

func (duplicateEventAgent) Run(ctx context.Context, turn Turn, emit func(Event)) (string, error) {
	emit(DeltaEvent("hello"))
	emit(DeltaEvent("hello"))
	emit(Event{Data: `{"status":"in_progress","title":"Run","toolCallId":"tool-1"}`, Type: EventToolCall})
	emit(Event{Data: `{"status":"in_progress","title":"Run","toolCallId":"tool-1"}`, Type: EventToolCall})
	emit(Event{Data: `{"status":"completed","title":"Run","toolCallId":"tool-1"}`, Type: EventToolCallUpdate})
	emit(Event{Data: `{"status":"completed","title":"Run","toolCallId":"tool-1"}`, Type: EventToolCallUpdate})
	emit(DeltaEvent("again"))
	emit(DeltaEvent("again"))
	return "", nil
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
