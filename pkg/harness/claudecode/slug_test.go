package claudecode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSlug(t *testing.T) {
	t.Run("sanitizeSlug", func(t *testing.T) {
		tests := []struct {
			in   string
			name string
			want string
		}{
			{name: "joins words with dashes", in: "fix login bug", want: "fix-login-bug"},
			{name: "keeps an existing kebab slug", in: "rename-feature-x", want: "rename-feature-x"},
			{name: "folds uppercase", in: "Fix Login Bug", want: "fix-login-bug"},
			{name: "strips punctuation", in: "feat: add login!?", want: "feat-add-login"},
			{name: "trims leading dashes", in: "  -- hello world", want: "hello-world"},
			{name: "trims trailing dashes", in: "hello world---", want: "hello-world"},
			{name: "cuts the response at a newline", in: "fix-login\nmore text after", want: "fix-login"},
			{name: "turns underscores into dashes", in: "fix_login_bug", want: "fix-login-bug"},
			{name: "caps the length", in: "this slug is going to be way too long for our liking", want: "this-slug-is-going-to-be-way-too"},
			{name: "returns empty for blank input", in: "   ", want: ""},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				a.Equal(tt.want, sanitizeSlug(tt.in))
			})
		}
	})

	t.Run("SlugForPrompt", func(t *testing.T) {
		t.Run("returns empty when the binary is missing", func(t *testing.T) {
			a := assert.New(t)
			a.Empty(SlugForPrompt("definitely-not-a-binary-xyz", "anything"))
		})

		t.Run("returns empty for an empty prompt", func(t *testing.T) {
			a := assert.New(t)
			a.Empty(SlugForPrompt("claude", ""))
		})
	})
}
