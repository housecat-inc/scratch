package chat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func claudeEvents(t *testing.T, textEmitted *bool, raw string) []Event {
	t.Helper()
	var line claudeLine
	require.NoError(t, json.Unmarshal([]byte(raw), &line))
	return claudeLineEvents(line, []byte(raw), textEmitted)
}

func TestClaudeLineEvents(t *testing.T) {
	tests := []struct {
		emitted bool
		line    string
		name    string
		want    []Event
	}{
		{
			name: "text delta",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}`,
			want: []Event{DeltaEvent("Hello")},
		},
		{
			name: "thinking delta",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm"}}}`,
			want: []Event{thinkingEvent("hmm")},
		},
		{
			name:    "text block separator",
			emitted: true,
			line:    `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text"}}}`,
			want:    []Event{DeltaEvent("\n\n")},
		},
		{
			name: "first text block no separator",
			line: `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text"}}}`,
			want: nil,
		},
		{
			name: "input json delta ignored",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{"}}}`,
			want: nil,
		},
		{
			name: "tool use",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"main.go"}}]}}`,
			want: []Event{{
				Data: `{"input":{"file_path":"main.go"},"status":"in_progress","title":"Read main.go","toolCallId":"toolu_1"}`,
				Type: EventToolCall,
			}},
		},
		{
			name: "tool result",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file body"}]}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"file body","type":"text"},"type":"content"}],"status":"completed","toolCallId":"toolu_1"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "tool result error",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":[{"type":"text","text":"denied"}]}]}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"denied","type":"text"},"type":"content"}],"status":"failed","toolCallId":"toolu_1"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "subagent lines skipped",
			line: `{"type":"assistant","parent_tool_use_id":"toolu_0","message":{"content":[{"type":"tool_use","id":"toolu_9","name":"Bash","input":{}}]}}`,
			want: nil,
		},
		{
			name: "result",
			line: `{"type":"result","subtype":"success","result":"done"}`,
			want: []Event{{Data: `{"type":"result","subtype":"success","result":"done"}`, Type: EventResult}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitted := tc.emitted
			assert.New(t).Equal(tc.want, claudeEvents(t, &emitted, tc.line))
		})
	}
}

func TestToolTitle(t *testing.T) {
	tests := []struct {
		input string
		name  string
		tool  string
		want  string
	}{
		{name: "command", tool: "Bash", input: `{"command":"ls -la"}`, want: "Bash ls -la"},
		{name: "file path", tool: "Read", input: `{"file_path":"a.go"}`, want: "Read a.go"},
		{name: "no primary", tool: "TodoWrite", input: `{"todos":[]}`, want: "TodoWrite"},
		{name: "empty input", tool: "Task", input: ``, want: "Task"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.New(t).Equal(tc.want, toolTitle(tc.tool, json.RawMessage(tc.input)))
		})
	}
}
