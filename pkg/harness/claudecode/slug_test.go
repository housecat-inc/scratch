package claudecode

import (
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeSlug(t *testing.T) {
	tk.Run(t, []tk.Test[string, string]{
		{Name: "basic kebab", In: "fix login bug", Out: "fix-login-bug"},
		{Name: "already kebab", In: "rename-feature-x", Out: "rename-feature-x"},
		{Name: "uppercase folded", In: "Fix Login Bug", Out: "fix-login-bug"},
		{Name: "strips punctuation", In: "feat: add login!?", Out: "feat-add-login"},
		{Name: "trims leading dashes", In: "  -- hello world", Out: "hello-world"},
		{Name: "trims trailing dashes", In: "hello world---", Out: "hello-world"},
		{Name: "newline cuts response", In: "fix-login\nmore text after", Out: "fix-login"},
		{Name: "underscore becomes dash", In: "fix_login_bug", Out: "fix-login-bug"},
		{Name: "max length", In: "this slug is going to be way too long for our liking", Out: "this-slug-is-going-to-be-way-too"},
		{Name: "empty", In: "   ", Out: ""},
	}, tk.Pure(sanitizeSlug))
}

func TestSlugForPromptNoBinary(t *testing.T) {
	a := assert.New(t)
	a.Empty(SlugForPrompt("definitely-not-a-binary-xyz", "anything"))
	a.Empty(SlugForPrompt("claude", ""))
}
