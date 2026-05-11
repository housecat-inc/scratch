package claudecode

import (
	"bytes"
	"os/exec"
	"strings"
	"time"
)

const slugMaxLen = 32

var slugPrompt = "Reply with ONLY a short kebab-case slug summarizing this task: 2-4 lowercase words separated by hyphens, max 32 characters, no other punctuation, no leading or trailing hyphens, no quotes, no explanation. Task: "

func SlugForPrompt(claudeBin, prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" || claudeBin == "" {
		return ""
	}
	if _, err := exec.LookPath(claudeBin); err != nil {
		return ""
	}

	cmd := exec.Command(claudeBin, "-p", "--model", "claude-haiku-4-5", slugPrompt+prompt)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &bytes.Buffer{}

	done := make(chan error, 1)
	go func() { done <- cmd.Run() }()
	select {
	case err := <-done:
		if err != nil {
			return ""
		}
	case <-time.After(15 * time.Second):
		_ = cmd.Process.Kill()
		return ""
	}

	return sanitizeSlug(out.String())
}

func sanitizeSlug(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, "\r\n"); i >= 0 {
		s = s[:i]
	}
	s = strings.ToLower(s)

	var b strings.Builder
	dashPending := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			if dashPending && b.Len() > 0 {
				b.WriteByte('-')
			}
			dashPending = false
			b.WriteRune(r)
		case r == '-' || r == '_' || r == ' ' || r == '.':
			if b.Len() > 0 {
				dashPending = true
			}
		}
		if b.Len() >= slugMaxLen {
			break
		}
	}
	return strings.TrimRight(b.String(), "-")
}
