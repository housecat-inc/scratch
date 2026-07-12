package db

import (
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/ts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThreadCRUD(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	defer restore()

	d := newTestDB(t)

	chat, err := d.AddThread(ThreadKindChat, "First chat", `{"agent":"echo"}`)
	r.NoError(err)
	a.NotZero(chat.ID)
	a.Equal(ThreadKindChat, chat.Kind)
	a.Equal(ThreadStateOpen, chat.State)
	a.Equal(`{"agent":"echo"}`, chat.Anchor)
	a.Equal("2026-07-11T12:00:00Z", chat.CreatedAt.Format(time.RFC3339))

	clock.Advance(time.Minute)
	review, err := d.AddThread(ThreadKindReview, "", "")
	r.NoError(err)
	a.Equal("{}", review.Anchor)

	chats, err := d.ListThreads(ThreadKindChat)
	r.NoError(err)
	r.Len(chats, 1)
	a.Equal(chat.ID, chats[0].ID)

	clock.Advance(time.Minute)
	second, err := d.AddThread(ThreadKindChat, "Second chat", "")
	r.NoError(err)

	chats, err = d.ListThreads(ThreadKindChat)
	r.NoError(err)
	r.Len(chats, 2)
	a.Equal(second.ID, chats[0].ID)

	clock.Advance(time.Minute)
	r.NoError(d.SetThreadTitle(chat.ID, "Renamed"))
	r.NoError(d.SetThreadState(chat.ID, ThreadStateResolved))
	r.NoError(d.SetThreadAnchor(chat.ID, `{"agent":"claude","session_id":"abc"}`))

	got, err := d.GetThread(chat.ID)
	r.NoError(err)
	a.Equal("Renamed", got.Title)
	a.Equal(ThreadStateResolved, got.State)
	a.Equal(`{"agent":"claude","session_id":"abc"}`, got.Anchor)

	chats, err = d.ListThreads(ThreadKindChat)
	r.NoError(err)
	a.Equal(chat.ID, chats[0].ID)

	clock.Advance(time.Minute)
	r.NoError(d.TouchThread(second.ID))
	got, err = d.GetThread(second.ID)
	r.NoError(err)
	a.Equal("2026-07-11T12:04:00Z", got.UpdatedAt.Format(time.RFC3339))
}

func TestThreadErrors(t *testing.T) {
	tests := []struct {
		act  func(d *DB) error
		name string
	}{
		{act: func(d *DB) error { return d.DeleteThread(404) }, name: "delete missing"},
		{act: func(d *DB) error { _, err := d.GetThread(404); return err }, name: "get missing"},
		{act: func(d *DB) error { return d.SetThreadAnchor(404, "{}") }, name: "anchor missing"},
		{act: func(d *DB) error { return d.SetThreadState(404, ThreadStateResolved) }, name: "state missing"},
		{act: func(d *DB) error { return d.SetThreadTitle(404, "x") }, name: "title missing"},
		{act: func(d *DB) error { return d.TouchThread(404) }, name: "touch missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			d := newTestDB(t)
			a.True(IsThreadNotFound(tc.act(d)))
		})
	}
}

func TestDeleteThreadDeletesMessagesAndEvents(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	d := newTestDB(t)

	thread, err := d.AddThread(ThreadKindChat, "chat", "")
	r.NoError(err)
	msg, err := d.AddMessage(NewMessage{
		Author:   "noah",
		Body:     "hello",
		Role:     MessageRoleUser,
		ThreadID: thread.ID,
	})
	r.NoError(err)
	_, err = d.AddMessageEvent(msg.ID, "delta", `{"text":"hello"}`)
	r.NoError(err)

	r.NoError(d.DeleteThread(thread.ID))

	_, err = d.GetThread(thread.ID)
	a.True(IsThreadNotFound(err))
	_, err = d.GetMessage(msg.ID)
	a.True(IsMessageNotFound(err))
	events, err := d.ListMessageEvents(msg.ID, 0)
	r.NoError(err)
	a.Empty(events)
}

func TestMessageLifecycle(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	clock, restore := ts.Mock(time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC))
	defer restore()

	d := newTestDB(t)

	thread, err := d.AddThread(ThreadKindChat, "chat", "")
	r.NoError(err)

	user, err := d.AddMessage(NewMessage{
		Author:   "noah",
		Body:     "hello agent",
		Role:     MessageRoleUser,
		ThreadID: thread.ID,
	})
	r.NoError(err)
	a.Equal(int64(1), user.Seq)
	a.Equal(MessageStatusComplete, user.Status)
	a.Equal("{}", user.Meta)
	a.Nil(user.CompletedAt)
	a.Nil(user.ParentID)

	asst, err := d.AddMessage(NewMessage{
		Author:   "agent:echo",
		Body:     "",
		ParentID: &user.ID,
		Role:     MessageRoleAssistant,
		Status:   MessageStatusStreaming,
		ThreadID: thread.ID,
	})
	r.NoError(err)
	a.Equal(int64(2), asst.Seq)
	r.NotNil(asst.ParentID)
	a.Equal(user.ID, *asst.ParentID)

	first, err := d.AddMessageEvent(asst.ID, "delta", `{"text":"hi "}`)
	r.NoError(err)
	a.Equal(int64(1), first.Seq)
	second, err := d.AddMessageEvent(asst.ID, "delta", `{"text":"there"}`)
	r.NoError(err)
	a.Equal(int64(2), second.Seq)

	r.NoError(d.AppendMessageBody(asst.ID, "hi "))
	r.NoError(d.AppendMessageBody(asst.ID, "there"))

	clock.Advance(time.Second)
	done, err := d.FinishMessage(asst.ID, MessageStatusComplete)
	r.NoError(err)
	a.Equal("hi there", done.Body)
	a.Equal(MessageStatusComplete, done.Status)
	r.NotNil(done.CompletedAt)
	a.Equal("2026-07-11T12:00:01Z", done.CompletedAt.Format(time.RFC3339))

	msgs, err := d.ListThreadMessages(thread.ID)
	r.NoError(err)
	r.Len(msgs, 2)
	a.Equal(MessageRoleUser, msgs[0].Role)
	a.Equal(MessageRoleAssistant, msgs[1].Role)

	events, err := d.ListMessageEvents(asst.ID, 0)
	r.NoError(err)
	r.Len(events, 2)
	a.Equal(`{"text":"hi "}`, events[0].Data)

	events, err = d.ListMessageEvents(asst.ID, 1)
	r.NoError(err)
	r.Len(events, 1)
	a.Equal(int64(2), events[0].Seq)

	other, err := d.AddThread(ThreadKindChat, "other", "")
	r.NoError(err)
	otherMsg, err := d.AddMessage(NewMessage{Author: "noah", Body: "x", Role: MessageRoleUser, ThreadID: other.ID})
	r.NoError(err)
	a.Equal(int64(1), otherMsg.Seq)
}

func TestMessageErrors(t *testing.T) {
	tests := []struct {
		act  func(d *DB) error
		name string
	}{
		{act: func(d *DB) error { return d.AppendMessageBody(404, "x") }, name: "append missing"},
		{act: func(d *DB) error { _, err := d.FinishMessage(404, MessageStatusComplete); return err }, name: "finish missing"},
		{act: func(d *DB) error { _, err := d.GetMessage(404); return err }, name: "get missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			d := newTestDB(t)
			a.True(IsMessageNotFound(tc.act(d)))
		})
	}
}
