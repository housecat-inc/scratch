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

func TestClaudecode(t *testing.T) {

	t.Run("when installing tmux on linux", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("linux-only")
		}

		tests := []struct {
			bins []string
			name string
			want []string
		}{
			{
				name: "uses apk",
				bins: []string{"apk"},
				want: []string{"apk", "add", "tmux"},
			},
			{
				name: "uses apt-get",
				bins: []string{"apt-get"},
				want: []string{"apt-get", "install", "-y", "tmux"},
			},
			{
				name: "uses dnf",
				bins: []string{"dnf"},
				want: []string{"dnf", "install", "-y", "tmux"},
			},
			{
				name: "uses pacman",
				bins: []string{"pacman"},
				want: []string{"pacman", "-S", "--noconfirm", "tmux"},
			},
			{
				name: "prefers apk over yum when both present",
				bins: []string{"apk", "yum"},
				want: []string{"apk", "add", "tmux"},
			},
			{
				name: "uses yum",
				bins: []string{"yum"},
				want: []string{"yum", "install", "-y", "tmux"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				dir := t.TempDir()
				for _, bin := range tt.bins {
					r.NoError(os.WriteFile(filepath.Join(dir, bin), []byte("#!/bin/sh\nexit 0\n"), 0o755))
				}
				t.Setenv("PATH", dir)

				got, err := tmuxInstallCommand()
				r.NoError(err)
				want := tt.want
				if os.Geteuid() != 0 {
					want = append([]string{"sudo", "-n"}, want...)
				}
				a.Equal(want, got)
			})
		}

		t.Run("errors without a package manager", func(t *testing.T) {
			a := assert.New(t)

			t.Setenv("PATH", t.TempDir())

			_, err := tmuxInstallCommand()
			a.Error(err)
		})
	})
}

func TestEnsureTmuxInstalled(t *testing.T) {
	t.Run("succeeds when tmux is already on the path", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		stub := filepath.Join(dir, "tmux")
		r.NoError(os.WriteFile(stub, []byte("#!/bin/sh\nexit 0\n"), 0o755))
		t.Setenv("PATH", dir)

		a.NoError(EnsureTmuxInstalled())
	})
}

func TestMergeDefaults(t *testing.T) {
	tests := []struct {
		defaults any
		existing string
		name     string
		want     string
	}{
		{
			name:     "writes every claude.json default to a new file",
			defaults: claudeJSONDefaults,
			want: `{
					"hasCompletedOnboarding": true,
					"hasUsedRemoteControl": true,
					"remoteDialogSeen": true
				}`,
		},
		{
			name:     "writes every settings.json default to a new file",
			defaults: settingsDefaults,
			want: `{
					"permissions": {"defaultMode": "bypassPermissions"},
					"skipDangerousModePermissionPrompt": true,
					"theme": "auto"
				}`,
		},
		{
			name:     "treats an empty file as an empty object",
			existing: "",
			defaults: settingsDefaults,
			want: `{
					"permissions": {"defaultMode": "bypassPermissions"},
					"skipDangerousModePermissionPrompt": true,
					"theme": "auto"
				}`,
		},
		{
			name:     "keeps an existing projects map",
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
			name:     "keeps an existing theme",
			existing: `{"theme": "dark"}`,
			defaults: settingsDefaults,
			want: `{
					"permissions": {"defaultMode": "bypassPermissions"},
					"skipDangerousModePermissionPrompt": true,
					"theme": "dark"
				}`,
		},
		{
			name:     "keeps a nested scalar inside permissions",
			existing: `{"permissions": {"defaultMode": "ask"}}`,
			defaults: settingsDefaults,
			want: `{
					"permissions": {"defaultMode": "ask"},
					"skipDangerousModePermissionPrompt": true,
					"theme": "auto"
				}`,
		},
		{
			name:     "keeps unknown fields",
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

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "config.json")
			if tt.existing != "" {
				r.NoError(os.WriteFile(path, []byte(tt.existing), 0o644))
			}

			r.NoError(MergeDefaults(path, tt.defaults))

			a.Equal(parseJSON(t, tt.want), readJSON(t, path))
		})
	}

	unchanged := []struct {
		defaults any
		existing any
		name     string
	}{
		{
			name:     "skips the rewrite when claude.json is complete",
			defaults: claudeJSONDefaults,
			existing: claudeJSONDefaults,
		},
		{
			name:     "skips the rewrite when extra fields already satisfy defaults",
			defaults: settingsDefaults,
			existing: map[string]any{
				"permissions":                       map[string]any{"defaultMode": "ask", "extra": true},
				"skipDangerousModePermissionPrompt": false,
				"theme":                             "dark",
				"unrelated":                         "value",
			},
		},
		{
			name:     "skips the rewrite when settings.json is complete",
			defaults: settingsDefaults,
			existing: settingsDefaults,
		},
	}

	for _, tt := range unchanged {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), "config.json")
			data, err := json.Marshal(tt.existing)
			r.NoError(err)
			r.NoError(os.WriteFile(path, data, 0o644))

			before, err := os.Stat(path)
			r.NoError(err)

			r.NoError(MergeDefaults(path, tt.defaults))

			after, err := os.Stat(path)
			r.NoError(err)
			a.Equal(before.ModTime(), after.ModTime(), "file rewritten despite no missing keys")
		})
	}

	t.Run("creates missing parent directories", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		path := filepath.Join(t.TempDir(), "nested", "dir", "settings.json")
		r.NoError(MergeDefaults(path, settingsDefaults))

		_, err := os.Stat(path)
		a.NoError(err)
	})
}

func TestTrustProjectAt(t *testing.T) {
	tests := []struct {
		dir      string
		existing string
		name     string
		want     string
	}{
		{
			name: "adds a trusted entry to a missing file",
			dir:  "/tmp/work",
			want: `{
					"projects": {"/tmp/work": {"hasTrustDialogAccepted": true}}
				}`,
		},
		{
			name:     "keeps other trusted projects",
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
			name:     "keeps other top-level fields",
			existing: `{"hasCompletedOnboarding": true, "oauthAccount": {"emailAddress": "x@y"}}`,
			dir:      "/tmp/work",
			want: `{
					"hasCompletedOnboarding": true,
					"oauthAccount": {"emailAddress": "x@y"},
					"projects": {"/tmp/work": {"hasTrustDialogAccepted": true}}
				}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			path := filepath.Join(t.TempDir(), ".claude.json")
			if tt.existing != "" {
				r.NoError(os.WriteFile(path, []byte(tt.existing), 0o644))
			}
			r.NoError(TrustProjectAt(path, tt.dir))
			a.Equal(parseJSON(t, tt.want), readJSON(t, path))
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
