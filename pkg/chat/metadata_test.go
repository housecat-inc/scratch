package chat

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadata(t *testing.T) {
	t.Parallel()

	t.Run("FriendlyTitle", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			in   string
			name string
			want string
		}{
			{in: "fix the mobile spacing around the composer", name: "uses the opening words of a plain prompt", want: "fix the mobile spacing around"},
			{in: "Selected html > body > div.mail-shell on http://localhost:8888/inbox", name: "labels a bare page selection", want: "Page selection"},
			{in: "Selected html > body on http://localhost:8888/inbox\nfix the spacing", name: "uses the instruction after a selection", want: "fix the spacing"},
			{in: "See attached files.", name: "labels an attachment-only prompt", want: "Attachment review"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				a := assert.New(t)

				a.Equal(tt.want, FriendlyTitle(tt.in))
			})
		}
	})

	t.Run("FriendlyDescription", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			in   string
			name string
			want string
		}{
			{in: "fix the mobile spacing. keep the header compact", name: "uses the first sentence of a plain prompt", want: "fix the mobile spacing."},
			{in: "Selected html > body > div.mail-shell on http://localhost:8888/inbox/chats/71?x=1", name: "describes a selection by its page path", want: "Selected page section on /inbox/chats/71?x=1"},
			{in: "Selected html > body", name: "keeps a selection without a url as is", want: "Selected html > body"},
			{in: "See attached files.", name: "describes an attachment-only prompt", want: "Reviewing attached files"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				a := assert.New(t)

				a.Equal(tt.want, FriendlyDescription(tt.in))
			})
		}
	})
}
