package claudecode

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeSlug(t *testing.T) {
	tests := []struct {
		in   string
		name string
		want string
	}{
		{name: "basic kebab", in: "fix login bug", want: "fix-login-bug"},
		{name: "already kebab", in: "rename-feature-x", want: "rename-feature-x"},
		{name: "uppercase folded", in: "Fix Login Bug", want: "fix-login-bug"},
		{name: "strips punctuation", in: "feat: add login!?", want: "feat-add-login"},
		{name: "trims leading dashes", in: "  -- hello world", want: "hello-world"},
		{name: "trims trailing dashes", in: "hello world---", want: "hello-world"},
		{name: "newline cuts response", in: "fix-login\nmore text after", want: "fix-login"},
		{name: "underscore becomes dash", in: "fix_login_bug", want: "fix-login-bug"},
		{name: "max length", in: "this slug is going to be way too long for our liking", want: "this-slug-is-going-to-be-way-too"},
		{name: "empty", in: "   ", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tc.want, sanitizeSlug(tc.in))
		})
	}
}

func TestSlugForPromptNoBinary(t *testing.T) {
	a := assert.New(t)
	a.Empty(SlugForPrompt("definitely-not-a-binary-xyz", "anything"))
	a.Empty(SlugForPrompt("claude", ""))
}
