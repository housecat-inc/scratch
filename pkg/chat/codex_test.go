package chat

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func codexEvents(t *testing.T, textEmitted *bool, raw string) []Event {
	t.Helper()
	var line codexLine
	require.NoError(t, json.Unmarshal([]byte(raw), &line))
	return codexItemEvents(line.Type, line.Item, textEmitted)
}

func TestCodexArgs(t *testing.T) {
	tests := []struct {
		name  string
		state codexAnchor
		want  []string
	}{
		{
			name:  "starts a new session with full access",
			state: codexAnchor{},
			want:  []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "danger-full-access", "hi"},
		},
		{
			name:  "passes the model to a new session",
			state: codexAnchor{Model: "gpt-5.2-codex"},
			want:  []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "danger-full-access", "--model", "gpt-5.2-codex", "hi"},
		},
		{
			name:  "resumes with a config override for the sandbox",
			state: codexAnchor{ThreadID: "t-1"},
			want:  []string{"exec", "resume", "--json", "--skip-git-repo-check", "-c", `sandbox_mode="danger-full-access"`, "t-1", "hi"},
		},
		{
			name:  "keeps safe mode read-only",
			state: codexAnchor{Access: AccessSafe},
			want:  []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "read-only", "hi"},
		},
		{
			name:  "keeps a safe resume read-only",
			state: codexAnchor{Access: AccessSafe, ThreadID: "t-1"},
			want:  []string{"exec", "resume", "--json", "--skip-git-repo-check", "-c", `sandbox_mode="read-only"`, "t-1", "hi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			dir := t.TempDir()
			tt.want[len(tt.want)-1] = agentPrompt(Turn{Prompt: "hi"}, dir, nil)
			a.Equal(tt.want, codexArgs(tt.state, Turn{Prompt: "hi"}, dir))
		})
	}

	t.Run("does not add workspace write roots", func(t *testing.T) {
		a := assert.New(t)

		dir := t.TempDir()

		args := codexArgs(codexAnchor{}, Turn{Prompt: "hi"}, dir)
		a.Contains(args, "danger-full-access")
		a.NotContains(args, `sandbox_workspace_write.writable_roots=["/repos/scratch/.git"]`)

		safe := codexArgs(codexAnchor{Access: AccessSafe}, Turn{Prompt: "hi"}, dir)
		a.Contains(safe, "read-only")
		a.NotContains(safe, `sandbox_workspace_write.writable_roots=["/repos/scratch/.git"]`)
	})

	t.Run("passes images as flags and lists other files in the prompt", func(t *testing.T) {
		a := assert.New(t)

		turn := Turn{
			Attachments: []Attachment{
				{MimeType: "image/png", Path: "/tmp/shot.png"},
				{MimeType: "text/csv", Path: "/tmp/data.csv"},
			},
			Prompt: "look",
		}
		args := codexArgs(codexAnchor{}, turn, t.TempDir())
		a.Contains(args, "--image=/tmp/shot.png")
		a.Contains(args[len(args)-1], "Do not create, switch, rename, or reset git branches")
		a.Contains(args[len(args)-1], "User request:\nlook")
		a.Contains(args[len(args)-1], "Attached files:\n- /tmp/data.csv")
	})
}

func TestCodexItemEvents(t *testing.T) {
	tests := []struct {
		emitted bool
		line    string
		name    string
		want    []Event
	}{
		{
			name: "emits an agent message as text",
			line: `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"done"}}`,
			want: []Event{DeltaEvent("done")},
		},
		{
			name:    "separates later agent messages with a blank line",
			emitted: true,
			line:    `{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"more"}}`,
			want:    []Event{DeltaEvent("\n\nmore")},
		},
		{
			name: "emits reasoning as thinking",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking about it"}}`,
			want: []Event{thinkingEvent("thinking about it\n\n")},
		},
		{
			name: "starts a tool call when a command starts",
			line: `{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"ls","status":"in_progress"}}`,
			want: []Event{{Data: `{"status":"in_progress","title":"ls","toolCallId":"item_0"}`, Type: EventToolCall}},
		},
		{
			name: "completes a tool call with the command output",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"a.go","exit_code":0,"status":"completed"}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"a.go","type":"text"},"type":"content"}],"status":"completed","title":"ls","toolCallId":"item_0"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "fails a tool call when the command fails",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"false","exit_code":1,"status":"failed"}}`,
			want: []Event{{Data: `{"status":"failed","title":"false","toolCallId":"item_0"}`, Type: EventToolCallUpdate}},
		},
		{
			name: "ignores partial command output",
			line: `{"type":"item.updated","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"partial"}}`,
			want: nil,
		},
		{
			name: "titles a file change by kind and path",
			line: `{"type":"item.completed","item":{"id":"item_3","type":"file_change","changes":[{"kind":"add","path":"a.go"}],"status":"completed"}}`,
			want: []Event{{Data: `{"status":"completed","title":"Edit: add a.go","toolCallId":"item_3"}`, Type: EventToolCall}},
		},
		{
			name: "turns a todo list into a plan",
			line: `{"type":"item.updated","item":{"id":"item_4","type":"todo_list","items":[{"text":"one","completed":true},{"text":"two","completed":false}]}}`,
			want: []Event{{
				Data: `{"entries":[{"content":"one","status":"completed"},{"content":"two","status":"pending"}]}`,
				Type: EventPlan,
			}},
		},
		{
			name: "titles a web search by its query",
			line: `{"type":"item.completed","item":{"id":"item_5","type":"web_search","query":"go generics"}}`,
			want: []Event{{Data: `{"status":"completed","title":"Search: go generics","toolCallId":"item_5"}`, Type: EventToolCall}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)

			emitted := tt.emitted
			a.Equal(tt.want, codexEvents(t, &emitted, tt.line))
		})
	}
}
