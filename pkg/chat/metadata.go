package chat

import (
	"net/url"
	"strings"
)

const (
	attachmentPrompt = "See attached files."
	pageSelection    = "Page selection"
	titleMaxWords    = 5
)

func FriendlyDescription(prompt string) string {
	prompt = strings.TrimSpace(prompt)
	if prompt == "" {
		return ""
	}
	if sentence := firstMeaningfulPromptSentence(prompt); sentence != "" {
		return truncate(sentence, titleMaxLen)
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
		return maxWords(line, titleMaxWords)
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

func firstMeaningfulPromptSentence(prompt string) string {
	line := firstMeaningfulPromptLine(prompt)
	if line == "" {
		return ""
	}
	for _, sep := range []string{". ", "! ", "? "} {
		if idx := strings.Index(line, sep); idx >= 0 {
			return strings.TrimSpace(line[:idx+1])
		}
	}
	return line
}

func maxWords(value string, limit int) string {
	words := strings.Fields(value)
	if len(words) <= limit {
		return strings.TrimSpace(value)
	}
	return strings.Join(words[:limit], " ")
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
