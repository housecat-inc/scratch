package flow

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stack struct {
	Flows *Flows
	Store *db.DB
	Svc   *chat.Service
}

func newStack(t *testing.T) *stack {
	t.Helper()
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "scratch.db")
	store, err := db.New(path)
	r.NoError(err)
	t.Cleanup(func() { store.Close() })

	workflows, err := workflow.New(path)
	r.NoError(err)

	svc := chat.NewService(store, chat.EchoAgent{Delay: time.Millisecond}, nil)
	flows := New(Deps{DBOS: workflows.Ctx(), Publish: svc.Publish, Store: store, Tasks: store})
	svc.RegisterAgent("contact", flows.Agent())
	svc.SetResolver(flows)
	r.NoError(workflows.Launch())

	t.Cleanup(func() {
		svc.Close()
		workflows.Close()
	})
	return &stack{Flows: flows, Store: store, Svc: svc}
}

func waitForm(t *testing.T, svc *chat.Service, threadID int64) (int64, elicit.Prompt) {
	t.Helper()
	var messageID int64
	var prompt elicit.Prompt
	require.Eventually(t, func() bool {
		view, err := svc.View(threadID)
		if err != nil {
			return false
		}
		for id, p := range view.Forms {
			messageID, prompt = id, p
			return true
		}
		return false
	}, 10*time.Second, 20*time.Millisecond)
	return messageID, prompt
}

func waitComplete(t *testing.T, svc *chat.Service, threadID int64) chat.ThreadView {
	t.Helper()
	var view chat.ThreadView
	require.Eventually(t, func() bool {
		v, err := svc.View(threadID)
		if err != nil {
			return false
		}
		view = v
		return !v.Streaming
	}, 10*time.Second, 20*time.Millisecond)
	return view
}

func TestContactIntakeAccept(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s := newStack(t)
	thread, err := s.Svc.CreateThread("contact", "")
	r.NoError(err)

	_, err = s.Svc.Send(thread.ID, "Add Jane Doe jane@example.com from ACME")
	r.NoError(err)

	messageID, prompt := waitForm(t, s.Svc, thread.ID)
	a.Equal("Review the contact before I save it.", prompt.Message)
	a.Equal([]string{"company", "email", "name", "notes"}, prompt.Order)
	a.Equal(`"jane@example.com"`, string(prompt.RequestedSchema.Properties["email"].Default))
	a.Equal(`"Jane Doe"`, string(prompt.RequestedSchema.Properties["name"].Default))

	err = s.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionAccept, map[string]string{
		"company": "ACME",
		"email":   "jane@example.com",
		"name":    "Jane Doe",
		"notes":   "Met at the conference",
	})
	r.NoError(err)

	err = s.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionAccept, map[string]string{
		"email": "jane@example.com",
		"name":  "Jane Doe",
	})
	a.ErrorIs(err, chat.ErrElicitationResolved)

	view := waitComplete(t, s.Svc, thread.ID)
	r.Len(view.Messages, 2)
	asst := view.Messages[1]
	a.Equal(db.MessageStatusComplete, asst.Status)
	a.Contains(asst.Body, "Added todo #1: Follow up with Jane Doe <jane@example.com>")
	a.Equal([]string{"draft_contact", "add_task"}, view.ToolCalls[asst.ID])
	a.Empty(view.Forms)

	tasks, err := s.Store.ListTasks()
	r.NoError(err)
	r.Len(tasks, 1)
	a.Equal("Follow up with Jane Doe <jane@example.com>", tasks[0].Title)
}

func TestContactIntakeDecline(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s := newStack(t)
	thread, err := s.Svc.CreateThread("contact", "")
	r.NoError(err)

	_, err = s.Svc.Send(thread.ID, "met bob@example.com")
	r.NoError(err)

	messageID, prompt := waitForm(t, s.Svc, thread.ID)
	r.NoError(s.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionDecline, nil))

	view := waitComplete(t, s.Svc, thread.ID)
	a.Contains(view.Messages[1].Body, "discarded the contact draft")

	tasks, err := s.Store.ListTasks()
	r.NoError(err)
	a.Empty(tasks)
}

func TestContactIntakeRejectsInvalidInput(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	s := newStack(t)
	thread, err := s.Svc.CreateThread("contact", "")
	r.NoError(err)

	_, err = s.Svc.Send(thread.ID, "Add Jane Doe jane@example.com")
	r.NoError(err)

	messageID, prompt := waitForm(t, s.Svc, thread.ID)
	err = s.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionAccept, map[string]string{"name": "Jane"})
	a.True(elicit.IsInvalid(err))

	r.NoError(s.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionDecline, nil))
	waitComplete(t, s.Svc, thread.ID)
}

func TestDraftContact(t *testing.T) {
	tests := []struct {
		name   string
		prompt string
		want   Contact
	}{
		{
			name:   "name and email",
			prompt: "Add Jane Doe jane@example.com from ACME",
			want:   Contact{Email: "jane@example.com", Name: "Jane Doe"},
		},
		{
			name:   "email only",
			prompt: "met bob@example.com",
			want:   Contact{Email: "bob@example.com"},
		},
		{
			name:   "no matches",
			prompt: "someone i met yesterday",
			want:   Contact{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.New(t).Equal(tc.want, draftContact(tc.prompt))
		})
	}
}
