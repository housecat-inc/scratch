package chat

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachmentLifecycle(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{Delay: time.Millisecond})
	thread, err := svc.CreateThread("", "files")
	r.NoError(err)

	attachment, err := svc.AddAttachment(thread.ID, "shot.png", "image/png", strings.NewReader("png-bytes"))
	r.NoError(err)
	t.Cleanup(func() { os.Remove(attachment.FilePath) })
	a.Equal("shot.png", attachment.Name)
	a.Equal(int64(len("png-bytes")), attachment.Size)
	a.Nil(attachment.MessageID)
	data, err := os.ReadFile(attachment.FilePath)
	r.NoError(err)
	a.Equal("png-bytes", string(data))

	user, err := svc.Send(thread.ID, "look at this", attachment.ID)
	r.NoError(err)
	waitComplete(t, svc, thread.ID)

	view, err := svc.View(thread.ID)
	r.NoError(err)
	r.Len(view.Attachments[user.ID], 1)
	a.Equal(attachment.ID, view.Attachments[user.ID][0].ID)

	err = svc.DeleteDraftAttachment(attachment.ID)
	a.True(db.IsAttachmentNotFound(err))
}

func TestSendRejectsInvalidAttachment(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{})
	thread, err := svc.CreateThread("", "files")
	r.NoError(err)

	_, err = svc.Send(thread.ID, "look", 999)
	a.True(db.IsAttachmentNotFound(err))

	view, err := svc.View(thread.ID)
	r.NoError(err)
	a.Empty(view.Messages)
}

func TestDeleteDraftAttachmentRemovesFile(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{})
	thread, err := svc.CreateThread("", "drafts")
	r.NoError(err)

	attachment, err := svc.AddAttachment(thread.ID, "notes.txt", "text/plain", strings.NewReader("draft"))
	r.NoError(err)

	r.NoError(svc.DeleteDraftAttachment(attachment.ID))
	_, err = os.Stat(attachment.FilePath)
	a.True(os.IsNotExist(err))
}

func TestAddAttachmentRejectsOversize(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{})
	thread, err := svc.CreateThread("", "large")
	r.NoError(err)

	attachment, err := svc.AddAttachment(thread.ID, "large.bin", "application/octet-stream", strings.NewReader(strings.Repeat("x", attachmentMaxBytes+1)))
	a.True(errors.Is(err, ErrAttachmentTooLarge))
	a.Empty(attachment.FilePath)
}

func TestSanitizeFilename(t *testing.T) {
	tk.Run(t, []tk.Test[string, string]{
		{Name: "plain", In: "shot.png", Out: "shot.png"},
		{Name: "traversal", In: "../../etc/passwd", Out: "passwd"},
		{Name: "blank", In: "  ", Out: "attachment"},
		{Name: "absolute", In: "/tmp/x.txt", Out: "x.txt"},
	}, tk.Pure(sanitizeFilename))
}

func TestToggleThreadAccess(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	svc := newTestService(t, EchoAgent{})
	thread, err := svc.CreateThread("", "access")
	r.NoError(err)
	a.Equal(AccessFull, svc.ThreadAccess(thread))

	access, err := svc.ToggleThreadAccess(thread.ID)
	r.NoError(err)
	a.Equal(AccessSafe, access)

	thread, err = svc.Thread(thread.ID)
	r.NoError(err)
	a.Equal(AccessSafe, svc.ThreadAccess(thread))
	a.Contains(thread.Anchor, `"agent"`)

	access, err = svc.ToggleThreadAccess(thread.ID)
	r.NoError(err)
	a.Equal(AccessFull, access)
}

func TestClaudeArgsAccessAndAttachments(t *testing.T) {
	a := assert.New(t)
	dir := t.TempDir()

	full := claudeArgs(claudeAnchor{}, Turn{Prompt: "hi"}, dir)
	a.Contains(full, "bypassPermissions")
	a.Contains(full, "--append-system-prompt")
	a.Contains(full, agentInstructions(dir))

	safe := claudeArgs(claudeAnchor{Access: AccessSafe}, Turn{
		Attachments: []Attachment{{MimeType: "image/png", Path: "/tmp/shot.png"}},
		Prompt:      "hi",
	}, dir)
	a.Contains(safe, "default")
	a.NotContains(safe, "bypassPermissions")
	a.Contains(safe[1], "Attached files:\n- /tmp/shot.png")
}
