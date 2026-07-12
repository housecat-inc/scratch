package ui

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChatPartGroups(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	parts := []ChatPartProps{
		{Kind: "text", Text: "working"},
		{Kind: "tool", Tool: &ChatToolCallProps{ID: "one", Status: "completed", Title: "Read a.go"}},
		{Kind: "tool", Tool: &ChatToolCallProps{ID: "two", Status: "failed", Title: "Test"}},
		{Kind: "text", Text: "done"},
		{Kind: "tool", Tool: &ChatToolCallProps{ID: "three", Status: "in_progress", Title: "Build"}},
	}

	groups := chatPartGroups(parts)

	r.Len(groups, 4)
	a.Equal("text", groups[0].Kind)
	a.Equal("tools", groups[1].Kind)
	a.Equal("1 failed, 1 completed", groups[1].Summary)
	r.Len(groups[1].Tools, 2)
	a.Equal("text", groups[2].Kind)
	a.Equal("tool", groups[3].Kind)
	a.True(groups[3].Last)
}

func TestChatToolGroupSummary(t *testing.T) {
	a := assert.New(t)

	tools := []ChatPartProps{
		{Kind: "tool", Tool: &ChatToolCallProps{Status: "in_progress"}},
		{Kind: "tool", Tool: &ChatToolCallProps{Status: "pending"}},
		{Kind: "tool", Tool: &ChatToolCallProps{Status: "failed"}},
		{Kind: "tool", Tool: &ChatToolCallProps{Status: "completed"}},
		{Kind: "tool", Tool: &ChatToolCallProps{Status: "completed"}},
	}

	a.Equal("2 running, 1 failed, 2 completed", chatToolGroupSummary(tools))
}
