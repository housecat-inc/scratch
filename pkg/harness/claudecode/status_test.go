package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthenticatedAt(t *testing.T) {
	future := time.Now().Add(time.Hour).UnixMilli()
	past := time.Now().Add(-time.Hour).UnixMilli()

	tests := []struct {
		name    string
		write   *string
		want    bool
		wantErr bool
	}{
		{name: "missing file is not authenticated"},
		{name: "empty file is not authenticated", write: ptr("")},
		{
			name:  "valid token in future is authenticated",
			write: ptr(`{"claudeAiOauth": {"accessToken": "tok", "expiresAt": ` + itoa(future) + `}}`),
			want:  true,
		},
		{
			name:  "expired token is not authenticated",
			write: ptr(`{"claudeAiOauth": {"accessToken": "tok", "expiresAt": ` + itoa(past) + `}}`),
		},
		{
			name:  "missing access token is not authenticated",
			write: ptr(`{"claudeAiOauth": {"expiresAt": ` + itoa(future) + `}}`),
		},
		{
			name:    "malformed json errors",
			write:   ptr(`{not json`),
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), ".credentials.json")
			if tc.write != nil {
				r.NoError(os.WriteFile(path, []byte(*tc.write), 0o600))
			}

			ok, err := AuthenticatedAt(path)
			if tc.wantErr {
				a.Error(err)
				return
			}
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}

func ptr(s string) *string { return &s }

func TestConfiguredAt(t *testing.T) {
	tests := []struct {
		name        string
		claudeJSON  any
		settings    any
		writeClaude bool
		writeSet    bool
		want        bool
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
			name:        "both complete",
			claudeJSON:  claudeJSONDefaults,
			settings:    settingsDefaults,
			writeClaude: true,
			writeSet:    true,
			want:        true,
		},
		{
			name:        "claude.json missing key",
			claudeJSON:  map[string]any{"theme": "auto"},
			settings:    settingsDefaults,
			writeClaude: true,
			writeSet:    true,
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

			ok, err := ConfiguredAt(home)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
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
			existing: `{"permissions": {"defaultMode": "bypassPermissions"}, "skipDangerousModePermissionPrompt": true}`,
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
			existing: `{"permissions": {"defaultMode": "ask"}, "skipDangerousModePermissionPrompt": false}`,
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

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
