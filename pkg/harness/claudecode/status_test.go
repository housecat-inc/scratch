package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticated(t *testing.T) {
	tests := []struct {
		exit    int
		name    string
		stdout  string
		want    bool
		wantErr bool
	}{
		{name: "logged in", stdout: `{"loggedIn": true, "email": "noah@housecat.com"}`, want: true},
		{name: "logged out", stdout: `{"loggedIn": false}`},
		{name: "cli error is not authenticated", exit: 1, stdout: `not logged in`},
		{name: "malformed json errors", stdout: `{not json`, wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "out")
			r.NoError(os.WriteFile(path, []byte(tc.stdout), 0o644))
			cmd := exec.Command("sh", "-c", fmt.Sprintf("cat %q; exit %d", path, tc.exit))

			ok, err := authenticated(cmd)
			if tc.wantErr {
				a.Error(err)
				return
			}
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}

func TestConfiguredAt(t *testing.T) {
	tests := []struct {
		name          string
		claudeJSON    any
		settings      any
		writeAgents   bool
		writeClaude   bool
		writeSet      bool
		writeSymlinks bool
		want          bool
	}{
		{name: "neither file present"},
		{
			name:        "only claude.json complete",
			claudeJSON:  claudeJSONDefaults,
			writeClaude: true,
		},
		{
			name:     "only settings complete",
			settings: settingsDefaults,
			writeSet: true,
		},
		{
			name:        "settings complete but agents missing",
			claudeJSON:  claudeJSONDefaults,
			settings:    settingsDefaults,
			writeClaude: true,
			writeSet:    true,
		},
		{
			name:        "agents installed but symlinks missing",
			claudeJSON:  claudeJSONDefaults,
			settings:    settingsDefaults,
			writeAgents: true,
			writeClaude: true,
			writeSet:    true,
		},
		{
			name:          "all four complete",
			claudeJSON:    claudeJSONDefaults,
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writeSet:      true,
			writeSymlinks: true,
			want:          true,
		},
		{
			name:          "claude.json missing key",
			claudeJSON:    map[string]any{"theme": "auto"},
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writeSet:      true,
			writeSymlinks: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			r.NoError(os.MkdirAll(filepath.Join(home, ".claude"), 0o755))

			if tc.writeClaude {
				data, err := json.Marshal(tc.claudeJSON)
				r.NoError(err)
				r.NoError(os.WriteFile(filepath.Join(home, ".claude.json"), data, 0o644))
			}
			if tc.writeSet {
				data, err := json.Marshal(tc.settings)
				r.NoError(err)
				r.NoError(os.WriteFile(filepath.Join(home, ".claude", "settings.json"), data, 0o644))
			}
			if tc.writeAgents {
				r.NoError(os.MkdirAll(filepath.Join(home, "scratch", ".git"), 0o755))
			}
			if tc.writeSymlinks {
				r.NoError(EnsureSymlinksAt(home))
			}

			ok, err := ConfiguredAt(home)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}

func TestConfiguredAfterClaudeRewrite(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	home := t.TempDir()
	r.NoError(os.MkdirAll(filepath.Join(home, ".claude"), 0o755))

	r.NoError(MergeDefaults(filepath.Join(home, ".claude.json"), claudeJSONDefaults))
	r.NoError(MergeDefaults(filepath.Join(home, ".claude", "settings.json"), settingsDefaults))
	r.NoError(os.MkdirAll(filepath.Join(home, "scratch", ".git"), 0o755))
	r.NoError(EnsureSymlinksAt(home))

	rewritten := `{
		"hasCompletedOnboarding": true,
		"hasUsedRemoteControl": true,
		"projects": {"/tmp/work": {"hasTrustDialogAccepted": true}},
		"remoteDialogSeen": true,
		"installMethod": "native",
		"oauthAccount": {"emailAddress": "noah@housecat.com"}
	}`
	r.NoError(os.WriteFile(filepath.Join(home, ".claude.json"), []byte(rewritten), 0o644))

	ok, err := ConfiguredAt(home)
	r.NoError(err)
	a.True(ok, "Configured should stay true after claude rewrites .claude.json")
}

func TestHasDefaults(t *testing.T) {
	tests := []struct {
		defaults any
		existing string
		name     string
		want     bool
	}{
		{name: "missing file has no defaults", defaults: settingsDefaults},
		{
			name:     "complete file has defaults",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "bypassPermissions"}, "skipDangerousModePermissionPrompt": true, "theme": "auto"}`,
			want:     true,
		},
		{
			name:     "missing top-level key fails",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "bypassPermissions"}}`,
		},
		{
			name:     "different scalar still satisfies presence",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "ask"}, "skipDangerousModePermissionPrompt": false, "theme": "dark"}`,
			want:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "config.json")
			if tc.existing != "" {
				r.NoError(os.WriteFile(path, []byte(tc.existing), 0o644))
			}

			ok, err := HasDefaults(path, tc.defaults)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}
