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
			name: "emits text deltas",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}`,
			want: []Event{DeltaEvent("Hello")},
		},
		{
			name: "emits thinking deltas",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"thinking_delta","thinking":"hmm"}}}`,
			want: []Event{thinkingEvent("hmm")},
		},
		{
			name:    "separates later text blocks with a blank line",
			emitted: true,
			line:    `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text"}}}`,
			want:    []Event{DeltaEvent("\n\n")},
		},
		{
			name: "does not separate the first text block",
			line: `{"type":"stream_event","event":{"type":"content_block_start","content_block":{"type":"text"}}}`,
			want: nil,
		},
		{
			name: "ignores partial tool input",
			line: `{"type":"stream_event","event":{"type":"content_block_delta","delta":{"type":"input_json_delta","partial_json":"{"}}}`,
			want: nil,
		},
		{
			name: "starts a tool call titled from its input",
			line: `{"type":"assistant","message":{"content":[{"type":"tool_use","id":"toolu_1","name":"Read","input":{"file_path":"main.go"}}]}}`,
			want: []Event{{
				Data: `{"input":{"file_path":"main.go"},"status":"in_progress","title":"Read main.go","toolCallId":"toolu_1"}`,
				Type: EventToolCall,
			}},
		},
		{
			name: "completes a tool call with its result",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","content":"file body"}]}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"file body","type":"text"},"type":"content"}],"status":"completed","toolCallId":"toolu_1"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "fails a tool call whose result is an error",
			line: `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"toolu_1","is_error":true,"content":[{"type":"text","text":"denied"}]}]}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"denied","type":"text"},"type":"content"}],"status":"failed","toolCallId":"toolu_1"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "skips subagent lines",
			line: `{"type":"assistant","parent_tool_use_id":"toolu_0","message":{"content":[{"type":"tool_use","id":"toolu_9","name":"Bash","input":{}}]}}`,
			want: nil,
		},
		{
			name: "passes the final result through",
			line: `{"type":"result","subtype":"success","result":"done"}`,
			want: []Event{{Data: `{"type":"result","subtype":"success","result":"done"}`, Type: EventResult}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)

			emitted := tc.emitted
			a.Equal(tc.want, claudeEvents(t, &emitted, tc.line))
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
		{input: `{"command":"ls -la"}`, name: "appends the command", tool: "Bash", want: "Bash ls -la"},
		{input: `{"file_path":"a.go"}`, name: "appends the file path", tool: "Read", want: "Read a.go"},
		{input: `{"todos":[]}`, name: "uses the tool name without a primary input", tool: "TodoWrite", want: "TodoWrite"},
		{input: ``, name: "uses the tool name for empty input", tool: "Task", want: "Task"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			a.Equal(tt.want, toolTitle(tt.tool, json.RawMessage(tt.input)))
		})
	}
}
