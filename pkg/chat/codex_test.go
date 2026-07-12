package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
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
			name:  "new session",
			state: codexAnchor{},
			want:  []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "workspace-write", "hi"},
		},
		{
			name:  "new session with model",
			state: codexAnchor{Model: "gpt-5.2-codex"},
			want:  []string{"exec", "--json", "--skip-git-repo-check", "--sandbox", "workspace-write", "--model", "gpt-5.2-codex", "hi"},
		},
		{
			name:  "resume uses config override for sandbox",
			state: codexAnchor{ThreadID: "t-1"},
			want:  []string{"exec", "resume", "--json", "--skip-git-repo-check", "-c", `sandbox_mode="workspace-write"`, "t-1", "hi"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.New(t).Equal(tc.want, codexArgs(tc.state, Turn{Prompt: "hi"}, t.TempDir()))
		})
	}
}

func TestCodexArgsWorktreeWritableRoot(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	r.NoError(os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: /repos/scratch/.git/worktrees/colombo-v2\n"), 0o644))

	args := codexArgs(codexAnchor{}, Turn{Prompt: "hi"}, dir)
	a.Contains(args, `sandbox_workspace_write.writable_roots=["/repos/scratch/.git"]`)

	safe := codexArgs(codexAnchor{Access: AccessSafe}, Turn{Prompt: "hi"}, dir)
	a.Contains(safe, "read-only")
	a.NotContains(safe, `sandbox_workspace_write.writable_roots=["/repos/scratch/.git"]`)
}

func TestCodexArgsAttachments(t *testing.T) {
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
	a.Contains(args[len(args)-1], "Attached files:\n- /tmp/data.csv")
}

func TestWorktreeGitDir(t *testing.T) {
	tests := []struct {
		contents string
		name     string
		want     string
	}{
		{name: "worktree", contents: "gitdir: /repos/scratch/.git/worktrees/colombo-v2\n", want: "/repos/scratch/.git"},
		{name: "plain gitdir", contents: "gitdir: /repos/scratch/.git\n", want: "/repos/scratch/.git"},
		{name: "empty", contents: "", want: ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := require.New(t)
			dir := t.TempDir()
			r.NoError(os.WriteFile(filepath.Join(dir, ".git"), []byte(tc.contents), 0o644))
			assert.New(t).Equal(tc.want, worktreeGitDir(dir))
		})
	}
}

func TestWorktreeGitDirRelativePath(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	r.NoError(os.WriteFile(filepath.Join(dir, ".git"), []byte("gitdir: ../repo/.git/worktrees/colombo-v2\n"), 0o644))

	a.Equal(filepath.Clean(filepath.Join(dir, "../repo/.git")), worktreeGitDir(dir))
}

func TestWorktreeGitDirRegularRepo(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	r.NoError(os.Mkdir(filepath.Join(dir, ".git"), 0o755))
	a.Equal("", worktreeGitDir(dir))
}

func TestCodexItemEvents(t *testing.T) {
	tests := []struct {
		emitted bool
		line    string
		name    string
		want    []Event
	}{
		{
			name: "agent message",
			line: `{"type":"item.completed","item":{"id":"item_1","type":"agent_message","text":"done"}}`,
			want: []Event{DeltaEvent("done")},
		},
		{
			name:    "agent message separator",
			emitted: true,
			line:    `{"type":"item.completed","item":{"id":"item_2","type":"agent_message","text":"more"}}`,
			want:    []Event{DeltaEvent("\n\nmore")},
		},
		{
			name: "reasoning",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"reasoning","text":"thinking about it"}}`,
			want: []Event{thinkingEvent("thinking about it\n\n")},
		},
		{
			name: "command started",
			line: `{"type":"item.started","item":{"id":"item_0","type":"command_execution","command":"ls","status":"in_progress"}}`,
			want: []Event{{Data: `{"status":"in_progress","title":"ls","toolCallId":"item_0"}`, Type: EventToolCall}},
		},
		{
			name: "command completed",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"a.go","exit_code":0,"status":"completed"}}`,
			want: []Event{{
				Data: `{"content":[{"content":{"text":"a.go","type":"text"},"type":"content"}],"status":"completed","title":"ls","toolCallId":"item_0"}`,
				Type: EventToolCallUpdate,
			}},
		},
		{
			name: "command failed",
			line: `{"type":"item.completed","item":{"id":"item_0","type":"command_execution","command":"false","exit_code":1,"status":"failed"}}`,
			want: []Event{{Data: `{"status":"failed","title":"false","toolCallId":"item_0"}`, Type: EventToolCallUpdate}},
		},
		{
			name: "command update ignored",
			line: `{"type":"item.updated","item":{"id":"item_0","type":"command_execution","command":"ls","aggregated_output":"partial"}}`,
			want: nil,
		},
		{
			name: "file change",
			line: `{"type":"item.completed","item":{"id":"item_3","type":"file_change","changes":[{"kind":"add","path":"a.go"}],"status":"completed"}}`,
			want: []Event{{Data: `{"status":"completed","title":"Edit: add a.go","toolCallId":"item_3"}`, Type: EventToolCall}},
		},
		{
			name: "todo list",
			line: `{"type":"item.updated","item":{"id":"item_4","type":"todo_list","items":[{"text":"one","completed":true},{"text":"two","completed":false}]}}`,
			want: []Event{{
				Data: `{"entries":[{"content":"one","status":"completed"},{"content":"two","status":"pending"}]}`,
				Type: EventPlan,
			}},
		},
		{
			name: "web search",
			line: `{"type":"item.completed","item":{"id":"item_5","type":"web_search","query":"go generics"}}`,
			want: []Event{{Data: `{"status":"completed","title":"Search: go generics","toolCallId":"item_5"}`, Type: EventToolCall}},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			emitted := tc.emitted
			assert.New(t).Equal(tc.want, codexEvents(t, &emitted, tc.line))
		})
	}
}
