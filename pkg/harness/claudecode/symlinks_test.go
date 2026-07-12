package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureSymlinksAt(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, home string)
		wantErr string
	}{
		{
			name: "creates all symlinks from clean home",
		},
		{
			name: "idempotent when correct symlinks exist",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(EnsureSymlinksAt(home))
			},
		},
		{
			name: "replaces stale symlink",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(os.Symlink("nowhere", filepath.Join(home, "AGENTS.md")))
			},
		},
		{
			name: "creates parent dir for nested link",
			setup: func(t *testing.T, home string) {
				_, err := os.Stat(filepath.Join(home, ".claude"))
				require.New(t).True(os.IsNotExist(err))
			},
		},
		{
			name: "errors when target is a regular file",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(os.WriteFile(filepath.Join(home, "CLAUDE.md"), []byte("local"), 0o644))
			},
			wantErr: "is not a symlink",
		},
		{
			name: "errors when target is a regular directory",
			setup: func(t *testing.T, home string) {
				r := require.New(t)
				r.NoError(os.Mkdir(filepath.Join(home, ".claude"), 0o755))
				r.NoError(os.Mkdir(filepath.Join(home, ".claude", "skills"), 0o755))
			},
			wantErr: "is not a symlink",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, home)
			}

			err := EnsureSymlinksAt(home)
			if tc.wantErr != "" {
				a.ErrorContains(err, tc.wantErr)
				return
			}
			r.NoError(err)

			for name, src := range symlinkSources() {
				link := filepath.Join(home, name)
				want, err := symlinkTarget(home, name, src)
				r.NoError(err)
				got, err := os.Readlink(link)
				r.NoError(err)
				a.Equal(want, got)
			}
		})
	}
}

func TestHasSymlinksAt(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, home string)
		want  bool
	}{
		{
			name: "missing returns false",
			want: false,
		},
		{
			name: "all correct returns true",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(EnsureSymlinksAt(home))
			},
			want: true,
		},
		{
			name: "partial returns false",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(os.Symlink(filepath.Join("scratch", "AGENTS.md"), filepath.Join(home, "AGENTS.md")))
			},
			want: false,
		},
		{
			name: "wrong target returns false",
			setup: func(t *testing.T, home string) {
				r := require.New(t)
				r.NoError(EnsureSymlinksAt(home))
				r.NoError(os.Remove(filepath.Join(home, ".claude", "skills")))
				r.NoError(os.Symlink("elsewhere", filepath.Join(home, ".claude", "skills")))
			},
			want: false,
		},
		{
			name: "regular file returns false",
			setup: func(t *testing.T, home string) {
				require.New(t).NoError(os.WriteFile(filepath.Join(home, "AGENTS.md"), []byte("x"), 0o644))
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			if tc.setup != nil {
				tc.setup(t, home)
			}

			ok, err := HasSymlinksAt(home)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}
