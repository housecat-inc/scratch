package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFriendlyTitle(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{name: "plain prompt", prompt: "fix the mobile spacing", want: "fix the mobile spacing"},
		{name: "selection only", prompt: "Selected html > body > div.mail-shell on http://localhost:8888/inbox", want: "Page selection"},
		{name: "selection with instruction", prompt: "Selected html > body on http://localhost:8888/inbox\nfix the spacing", want: "fix the spacing"},
		{name: "attachment only", prompt: "See attached files.", want: "Attachment review"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)
			a.Equal(tt.want, FriendlyTitle(tt.prompt))
		})
	}
}

func TestFriendlyDescription(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		prompt string
		want   string
	}{
		{name: "plain prompt", prompt: "fix the mobile spacing", want: "fix the mobile spacing"},
		{name: "selection only", prompt: "Selected html > body > div.mail-shell on http://localhost:8888/inbox/chats/71?x=1", want: "Selected page section on /inbox/chats/71?x=1"},
		{name: "selection without url", prompt: "Selected html > body", want: "Selected html > body"},
		{name: "attachment only", prompt: "See attached files.", want: "Reviewing attached files"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			a := assert.New(t)
			a.Equal(tt.want, FriendlyDescription(tt.prompt))
		})
	}
}
