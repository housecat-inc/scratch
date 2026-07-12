package chat

import (
	"testing"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func events(pairs ...string) []db.MessageEvent {
	out := []db.MessageEvent{}
	for i := 0; i < len(pairs); i += 2 {
		out = append(out, db.MessageEvent{Data: pairs[i+1], Seq: int64(i / 2), Type: pairs[i]})
	}
	return out
}

func TestFoldPartsTextAndThinking(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := foldParts(events(
		EventThinking, `{"text":"hmm "}`,
		EventThinking, `{"text":"okay"}`,
		EventDelta, `{"text":"Hello "}`,
		EventDelta, `{"text":"world"}`,
		EventThinking, `{"text":"more thought"}`,
	), false)

	r.Len(parts, 3)
	a.Equal(PartThinking, parts[0].Kind)
	a.Equal("hmm okay", parts[0].Text)
	a.Equal(PartText, parts[1].Kind)
	a.Equal("Hello world", parts[1].Text)
	a.Equal(PartThinking, parts[2].Kind)
	a.Equal("more thought", parts[2].Text)
}

func TestFoldPartsToolCallMerge(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := foldParts(events(
		EventToolCallUpdate, `{"toolCallId":"tc-1","status":"in_progress","title":"Run tests"}`,
		EventDelta, `{"text":"Running now"}`,
		EventToolCallUpdate, `{"toolCallId":"tc-1","status":"completed","content":[{"type":"content","content":{"type":"text","text":"ok\t3 passed"}}]}`,
	), true)

	r.Len(parts, 2)
	r.Equal(PartTool, parts[0].Kind)
	a.Equal("Run tests", parts[0].Tool.Title)
	a.Equal(ToolStatusCompleted, parts[0].Tool.Status)
	a.Equal("ok\t3 passed", parts[0].Tool.Detail)
	a.Equal("Running now", parts[1].Text)
}

func TestFoldPartsToolDiffDetail(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := foldParts(events(
		EventToolCallUpdate, `{"toolCallId":"tc-2","title":"Edit main.go","content":[{"type":"diff","path":"main.go","oldText":"a\nb","newText":"c"}]}`,
	), true)

	r.Len(parts, 1)
	a.Equal("main.go\n- a\n- b\n+ c", parts[0].Tool.Detail)
	a.Equal(ToolStatusInProgress, parts[0].Tool.Status)
}

func TestFoldPartsLegacyToolCall(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := foldParts(events(
		EventToolCall, `{"name":"draft_contact","input":{"email":"jane@example.com"}}`,
	), false)

	r.Len(parts, 1)
	a.Equal("draft_contact", parts[0].Tool.Title)
	a.Equal(ToolStatusCompleted, parts[0].Tool.Status)
	a.Contains(parts[0].Tool.Detail, `"email": "jane@example.com"`)
}

func TestFoldPartsDanglingToolCompletesWithMessage(t *testing.T) {
	a := assert.New(t)

	streaming := foldParts(events(
		EventToolCallUpdate, `{"toolCallId":"tc-3","status":"in_progress","title":"Search"}`,
	), true)
	a.Equal(ToolStatusInProgress, streaming[0].Tool.Status)

	finished := foldParts(events(
		EventToolCallUpdate, `{"toolCallId":"tc-3","status":"in_progress","title":"Search"}`,
	), false)
	a.Equal(ToolStatusCompleted, finished[0].Tool.Status)
}

func TestFoldPartsPlanUpdatesInPlace(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := foldParts(events(
		EventPlan, `{"entries":[{"content":"one","status":"pending"}]}`,
		EventDelta, `{"text":"working"}`,
		EventPlan, `{"entries":[{"content":"one","status":"completed"},{"content":"two","status":"in_progress"}]}`,
	), true)

	r.Len(parts, 2)
	r.Equal(PartPlan, parts[0].Kind)
	r.Len(parts[0].Plan, 2)
	a.Equal("completed", parts[0].Plan[0].Status)
	a.Equal("two", parts[0].Plan[1].Content)
}
