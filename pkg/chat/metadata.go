package chat

import (
	"net/url"
	"strings"
)

const (
	attachmentPrompt = "See attached files."
	pageSelection    = "Page selection"
)

func FriendlyDescription(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if line := firstMeaningfulPromptLine(prompt); line != "" {
		return truncate(line, titleMaxLen)
	}
	if _, rawURL, ok := selectionNote(prompt); ok {
		if path := displayPath(rawURL); path != "" {
			return "Selected page section on " + path
		}
		return "Selected page section"
	}
	if strings.EqualFold(prompt, attachmentPrompt) {
		return "Reviewing attached files"
	}
	return truncate(prompt, titleMaxLen)
}

func FriendlyTitle(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if line := firstMeaningfulPromptLine(prompt); line != "" {
		return truncate(line, titleMaxLen)
	}
	if _, _, ok := selectionNote(prompt); ok {
		return pageSelection
	}
	if strings.EqualFold(prompt, attachmentPrompt) {
		return "Attachment review"
	}
	return truncate(prompt, titleMaxLen)
}

func displayPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if u.Path == "" {
		return "/"
	}
	if u.RawQuery != "" {
		return u.Path + "?" + u.RawQuery
	}
	return u.Path
}

func firstMeaningfulPromptLine(prompt string) string {
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.EqualFold(line, attachmentPrompt) {
			continue
		}
		if _, _, ok := selectionNote(line); ok {
			continue
		}
		if strings.HasPrefix(line, "<!--") {
			continue
		}
		return line
	}
	return ""
}

func selectionNote(prompt string) (string, string, bool) {
	for _, line := range strings.Split(prompt, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Selected ") {
			continue
		}
		rest := strings.TrimPrefix(line, "Selected ")
		for _, marker := range []string{" on https://", " on http://", " on /"} {
			if idx := strings.LastIndex(rest, marker); idx >= 0 {
				return strings.TrimSpace(rest[:idx]), strings.TrimSpace(rest[idx+4:]), true
			}
		}
	}
	return "", "", false
}
