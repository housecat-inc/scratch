package claudecode

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticated(t *testing.T) {
	type in struct {
		exit   int
		stdout string
	}

	tk.RunF(t, []tk.Test[in, bool]{
		{Name: "logged in", In: in{stdout: `{"loggedIn": true, "email": "noah@housecat.com"}`}, Out: true},
		{Name: "logged out", In: in{stdout: `{"loggedIn": false}`}, Out: false},
		{Name: "cli error is not authenticated", In: in{exit: 1, stdout: `not logged in`}, Out: false},
		{Name: "malformed json errors", In: in{stdout: `{not json`}, Assert: func(t *tk.T, _ bool, err error) { t.A.Error(err) }},
	}, func(t *tk.T) string { return t.TempDir() },
		func(t *tk.T, dir string, i in) (bool, error) {
			path := filepath.Join(dir, "out")
			t.R.NoError(os.WriteFile(path, []byte(i.stdout), 0o644))
			cmd := exec.Command("sh", "-c", fmt.Sprintf("cat %q; exit %d", path, i.exit))
			return authenticated(cmd)
		})
}

func TestConfiguredAt(t *testing.T) {
	tests := []struct {
		name          string
		claudeJSON    any
		settings      any
		writeAgents   bool
		writeClaude   bool
		writePath     bool
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
			name:          "symlinks done but path missing",
			claudeJSON:    claudeJSONDefaults,
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writeSet:      true,
			writeSymlinks: true,
		},
		{
			name:          "all five complete",
			claudeJSON:    claudeJSONDefaults,
			settings:      settingsDefaults,
			writeAgents:   true,
			writeClaude:   true,
			writePath:     true,
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
			writePath:     true,
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
			if tc.writePath {
				r.NoError(EnsureGoBinOnPathAt(home))
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
	r.NoError(EnsureGoBinOnPathAt(home))

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
