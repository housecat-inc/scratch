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

func TestStatus(t *testing.T) {
	t.Run("authenticated", func(t *testing.T) {
		tests := []struct {
			exit    int
			name    string
			stdout  string
			want    bool
			wantErr bool
		}{
			{name: "reports a logged in account", stdout: `{"loggedIn": true, "email": "noah@housecat.com"}`, want: true},
			{name: "reports a logged out account", stdout: `{"loggedIn": false}`},
			{name: "treats a cli error as not authenticated", exit: 1, stdout: `not logged in`},
			{name: "errors on malformed json", stdout: `{not json`, wantErr: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				path := filepath.Join(t.TempDir(), "out")
				r.NoError(os.WriteFile(path, []byte(tt.stdout), 0o644))
				cmd := exec.Command("sh", "-c", fmt.Sprintf("cat %q; exit %d", path, tt.exit))

				ok, err := authenticated(cmd)
				if tt.wantErr {
					a.Error(err)
					return
				}
				r.NoError(err)
				a.Equal(tt.want, ok)
			})
		}
	})

}

func TestConfiguredAt(t *testing.T) {
	tests := []struct {
		claudeJSON    any
		name          string
		settings      any
		want          bool
		writeAgents   bool
		writeClaude   bool
		writeSet      bool
		writeSymlinks bool
	}{
		{name: "is false when neither file exists"},
		{
			name:        "is false with only claude.json complete",
			claudeJSON:  claudeJSONDefaults,
			writeClaude: true,
		},
		{
			name:     "is false with only settings.json complete",
			settings: settingsDefaults,
			writeSet: true,
		},
		{
			name:        "is false when the agents checkout is missing",
			claudeJSON:  claudeJSONDefaults,
			settings:    settingsDefaults,
			writeClaude: true,
			writeSet:    true,
		},
		{
			name:        "is false when symlinks are missing",
			claudeJSON:  claudeJSONDefaults,
			settings:    settingsDefaults,
			writeAgents: true,
			writeClaude: true,
			writeSet:    true,
		},
		{
			name:          "is true when all four pieces are in place",
			claudeJSON:    claudeJSONDefaults,
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writeSet:      true,
			writeSymlinks: true,
			want:          true,
		},
		{
			name:          "is false when claude.json lacks a default key",
			claudeJSON:    map[string]any{"theme": "auto"},
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writeSet:      true,
			writeSymlinks: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			r.NoError(os.MkdirAll(filepath.Join(home, ".claude"), 0o755))

			if tt.writeClaude {
				data, err := json.Marshal(tt.claudeJSON)
				r.NoError(err)
				r.NoError(os.WriteFile(filepath.Join(home, ".claude.json"), data, 0o644))
			}
			if tt.writeSet {
				data, err := json.Marshal(tt.settings)
				r.NoError(err)
				r.NoError(os.WriteFile(filepath.Join(home, ".claude", "settings.json"), data, 0o644))
			}
			if tt.writeAgents {
				r.NoError(os.MkdirAll(filepath.Join(home, "scratch", ".git"), 0o755))
			}
			if tt.writeSymlinks {
				r.NoError(EnsureSymlinksAt(home))
			}

			ok, err := ConfiguredAt(home)
			r.NoError(err)
			a.Equal(tt.want, ok)
		})
	}

	t.Run("stays configured after claude rewrites claude.json", func(t *testing.T) {
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
	})
}

func TestHasDefaults(t *testing.T) {
	tests := []struct {
		defaults any
		existing string
		name     string
		want     bool
	}{
		{name: "is false for a missing file", defaults: settingsDefaults},
		{
			name:     "is true for a complete file",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "bypassPermissions"}, "skipDangerousModePermissionPrompt": true, "theme": "auto"}`,
			want:     true,
		},
		{
			name:     "is false when a top-level key is missing",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "bypassPermissions"}}`,
		},
		{
			name:     "is true when a key holds a different value",
			defaults: settingsDefaults,
			existing: `{"permissions": {"defaultMode": "ask"}, "skipDangerousModePermissionPrompt": false, "theme": "dark"}`,
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "config.json")
			if tt.existing != "" {
				r.NoError(os.WriteFile(path, []byte(tt.existing), 0o644))
			}

			ok, err := HasDefaults(path, tt.defaults)
			r.NoError(err)
			a.Equal(tt.want, ok)
		})
	}
}
