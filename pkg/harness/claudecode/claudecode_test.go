package claudecode

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeDefaults(t *testing.T) {
	tests := []struct {
		defaults any
		existing string
		name     string
		want     string
	}{
		{
			name:     "creates claude.json with all defaults",
			defaults: claudeJSONDefaults,
			want: `{
				"hasCompletedOnboarding": true,
				"hasUsedRemoteControl": true,
				"remoteDialogSeen": true
			}`,
		},
		{
			name:     "creates settings.json with all defaults",
			defaults: settingsDefaults,
			want: `{
				"permissions": {"defaultMode": "bypassPermissions"},
				"skipDangerousModePermissionPrompt": true,
				"theme": "auto"
			}`,
		},
		{
			name:     "empty file is treated as empty object",
			existing: "",
			defaults: settingsDefaults,
			want: `{
				"permissions": {"defaultMode": "bypassPermissions"},
				"skipDangerousModePermissionPrompt": true,
				"theme": "auto"
			}`,
		},
		{
			name:     "preserves existing projects map",
			existing: `{"projects": {"/home/me": {"hasTrustDialogAccepted": true}}}`,
			defaults: claudeJSONDefaults,
			want: `{
				"hasCompletedOnboarding": true,
				"hasUsedRemoteControl": true,
				"projects": {"/home/me": {"hasTrustDialogAccepted": true}},
				"remoteDialogSeen": true
			}`,
		},
		{
			name:     "preserves existing theme in settings",
			existing: `{"theme": "dark"}`,
			defaults: settingsDefaults,
			want: `{
				"permissions": {"defaultMode": "bypassPermissions"},
				"skipDangerousModePermissionPrompt": true,
				"theme": "dark"
			}`,
		},
		{
			name:     "preserves nested scalar inside permissions",
			existing: `{"permissions": {"defaultMode": "ask"}}`,
			defaults: settingsDefaults,
			want: `{
				"permissions": {"defaultMode": "ask"},
				"skipDangerousModePermissionPrompt": true,
				"theme": "auto"
			}`,
		},
		{
			name:     "preserves unknown fields not declared in struct",
			existing: `{"someFutureField": [1, 2, 3]}`,
			defaults: claudeJSONDefaults,
			want: `{
				"hasCompletedOnboarding": true,
				"hasUsedRemoteControl": true,
				"remoteDialogSeen": true,
				"someFutureField": [1, 2, 3]
			}`,
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

			r.NoError(MergeDefaults(path, tc.defaults))

			a.Equal(parseJSON(t, tc.want), readJSON(t, path))
		})
	}
}

func TestTrustProjectAt(t *testing.T) {
	tests := []struct {
		dir      string
		existing string
		test     string
		want     string
	}{
		{
			test: "creates entry on empty file",
			dir:  "/tmp/work",
			want: `{
				"projects": {"/tmp/work": {"hasTrustDialogAccepted": true}}
			}`,
		},
		{
			test:     "preserves existing trusted project",
			existing: `{"projects": {"/other": {"hasTrustDialogAccepted": true}}}`,
			dir:      "/tmp/work",
			want: `{
				"projects": {
					"/other": {"hasTrustDialogAccepted": true},
					"/tmp/work": {"hasTrustDialogAccepted": true}
				}
			}`,
		},
		{
			test:     "leaves other top-level fields alone",
			existing: `{"hasCompletedOnboarding": true, "oauthAccount": {"emailAddress": "x@y"}}`,
			dir:      "/tmp/work",
			want: `{
				"hasCompletedOnboarding": true,
				"oauthAccount": {"emailAddress": "x@y"},
				"projects": {"/tmp/work": {"hasTrustDialogAccepted": true}}
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.test, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), ".claude.json")
			if tc.existing != "" {
				r.NoError(os.WriteFile(path, []byte(tc.existing), 0o644))
			}
			r.NoError(TrustProjectAt(path, tc.dir))
			a.Equal(parseJSON(t, tc.want), readJSON(t, path))
		})
	}
}

func TestEnsureTmuxInstalledAlreadyOnPath(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	dir := t.TempDir()
	stub := filepath.Join(dir, "tmux")
	r.NoError(os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
	t.Setenv("PATH", dir)

	a.NoError(EnsureTmuxInstalled())
}

func TestTmuxInstallCommandLinux(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}

	tests := []struct {
		bins []string
		name string
		want []string
	}{
		{
			name: "apk",
			bins: []string{"apk"},
			want: []string{"apk", "add", "tmux"},
		},
		{
			name: "apt-get",
			bins: []string{"apt-get"},
			want: []string{"apt-get", "install", "-y", "tmux"},
		},
		{
			name: "dnf",
			bins: []string{"dnf"},
			want: []string{"dnf", "install", "-y", "tmux"},
		},
		{
			name: "pacman",
			bins: []string{"pacman"},
			want: []string{"pacman", "-S", "--noconfirm", "tmux"},
		},
		{
			name: "prefers apk over yum when both present",
			bins: []string{"apk", "yum"},
			want: []string{"apk", "add", "tmux"},
		},
		{
			name: "yum",
			bins: []string{"yum"},
			want: []string{"yum", "install", "-y", "tmux"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			dir := t.TempDir()
			for _, bin := range tc.bins {
				r.NoError(os.WriteFile(filepath.Join(dir, bin), []byte("#!/bin/sh\nexit 0\n"), 0o755))
			}
			t.Setenv("PATH", dir)

			got, err := tmuxInstallCommand()
			r.NoError(err)
			want := tc.want
			if os.Geteuid() != 0 {
				want = append([]string{"sudo", "-n"}, want...)
			}
			a.Equal(want, got)
		})
	}
}

func TestTmuxInstallCommandLinuxNoManager(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("linux-only")
	}
	a := assert.New(t)

	t.Setenv("PATH", t.TempDir())

	_, err := tmuxInstallCommand()
	a.Error(err)
}

func TestMergeDefaultsCreatesParentDir(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
	r.NoError(MergeDefaults(path, settingsDefaults))

	_, err := os.Stat(path)
	a.NoError(err)
}

func TestMergeDefaultsNoChangeNoRewrite(t *testing.T) {
	tests := []struct {
		defaults any
		existing any
		name     string
	}{
		{
			name:     "claude.json already complete",
			defaults: claudeJSONDefaults,
			existing: claudeJSONDefaults,
		},
		{
			name:     "extra fields present, defaults satisfied",
			defaults: settingsDefaults,
			existing: map[string]any{
				"permissions":                       map[string]any{"defaultMode": "ask", "extra": true},
				"skipDangerousModePermissionPrompt": false,
				"theme":                             "dark",
				"unrelated":                         "value",
			},
		},
		{
			name:     "settings already complete",
			defaults: settingsDefaults,
			existing: settingsDefaults,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "config.json")
			data, err := json.Marshal(tc.existing)
			r.NoError(err)
			r.NoError(os.WriteFile(path, data, 0o644))

			before, err := os.Stat(path)
			r.NoError(err)

			r.NoError(MergeDefaults(path, tc.defaults))

			after, err := os.Stat(path)
			r.NoError(err)
			a.Equal(before.ModTime(), after.ModTime(), "file rewritten despite no missing keys")
		})
	}
}

func parseJSON(t *testing.T, raw string) map[string]any {
	t.Helper()
	r := require.New(t)
	var out map[string]any
	r.NoError(json.Unmarshal([]byte(raw), &out))
	return out
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	r := require.New(t)
	data, err := os.ReadFile(path)
	r.NoError(err)
	var out map[string]any
	r.NoError(json.Unmarshal(data, &out))
	return out
}
