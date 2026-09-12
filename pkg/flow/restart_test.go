package flow

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type restartStack struct {
	Store *db.DB
	Svc   *chat.Service

	once      sync.Once
	workflows *workflow.Workflows
}

func (s *restartStack) Close() {
	s.once.Do(func() {
		s.Svc.Close()
		s.workflows.Close()
		s.Store.Close()
	})
}

func newRestartStack(t *testing.T, path string, extract Extractor) *restartStack {
	t.Helper()
	r := require.New(t)

	store, err := db.New(path)
	r.NoError(err)

	workflows, err := workflow.New(path)
	r.NoError(err)

	svc := chat.NewService(store, chat.EchoAgent{Delay: time.Millisecond}, nil)
	flows := New(Deps{DBOS: workflows.Ctx(), Extract: extract, Publish: svc.Publish, Store: store, Tasks: store})
	svc.RegisterAgent("contact", flows.Agent())
	svc.SetResolver(flows)
	r.NoError(workflows.Launch())
	r.NoError(svc.Recover())

	s := &restartStack{Store: store, Svc: svc, workflows: workflows}
	t.Cleanup(s.Close)
	return s
}

func TestContactIntakeSurvivesRestart(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "scratch.db")
	var extractions atomic.Int32
	extract := ExtractorFunc(func(ctx context.Context, message string) (Contact, error) {
		extractions.Add(1)
		return HeuristicExtractor().Extract(ctx, message)
	})

	first := newRestartStack(t, path, extract)
	thread, err := first.Svc.CreateThread("contact", "")
	r.NoError(err)
	_, err = first.Svc.Send(thread.ID, "Add Jane Doe jane@example.com from ACME")
	r.NoError(err)
	waitForm(t, first.Svc, thread.ID)
	first.Close()

	second := newRestartStack(t, path, extract)
	messageID, prompt := waitForm(t, second.Svc, thread.ID)
	a.Equal("Review the contact before I save it.", prompt.Message)

	err = second.Svc.ResolveElicitation(messageID, prompt.ElicitationID, elicit.ActionAccept, map[string]string{
		"email": "jane@example.com",
		"name":  "Jane Doe",
	})
	r.NoError(err)

	view := waitComplete(t, second.Svc, thread.ID)
	asst := view.Messages[1]
	a.Equal(db.MessageStatusComplete, asst.Status)
	a.Contains(asst.Body, "Follow up with Jane Doe <jane@example.com>")
	a.Equal(1, strings.Count(asst.Body, "Drafted a contact"))

	tasks, err := second.Store.ListTasks()
	r.NoError(err)
	r.Len(tasks, 1)
	a.Equal(int32(1), extractions.Load())
}
