package chat

import (
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
)

func TestFriendlyTitle(t *testing.T) {
	t.Parallel()

	tk.Run(t, []tk.Test[string, string]{
		{Name: "plain prompt", In: "fix the mobile spacing around the composer", Out: "fix the mobile spacing around"},
		{Name: "selection only", In: "Selected html > body > div.mail-shell on http://localhost:8888/inbox", Out: "Page selection"},
		{Name: "selection with instruction", In: "Selected html > body on http://localhost:8888/inbox\nfix the spacing", Out: "fix the spacing"},
		{Name: "attachment only", In: "See attached files.", Out: "Attachment review"},
	}, tk.Pure(FriendlyTitle), tk.Parallel())
}

func TestFriendlyDescription(t *testing.T) {
	t.Parallel()

	tk.Run(t, []tk.Test[string, string]{
		{Name: "plain prompt", In: "fix the mobile spacing. keep the header compact", Out: "fix the mobile spacing."},
		{Name: "selection only", In: "Selected html > body > div.mail-shell on http://localhost:8888/inbox/chats/71?x=1", Out: "Selected page section on /inbox/chats/71?x=1"},
		{Name: "selection without url", In: "Selected html > body", Out: "Selected html > body"},
		{Name: "attachment only", In: "See attached files.", Out: "Reviewing attached files"},
	}, tk.Pure(FriendlyDescription), tk.Parallel())
}
