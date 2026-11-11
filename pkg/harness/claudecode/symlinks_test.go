package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSymlinks(t *testing.T) {
	t.Run("EnsureSymlinksAt", func(t *testing.T) {
		tests := []struct {
			name    string
			setup   func(t *testing.T, home string)
			wantErr string
		}{
			{
				name: "creates every symlink in a clean home",
			},
			{
				name: "leaves correct symlinks in place",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(EnsureSymlinksAt(home))
				},
			},
			{
				name: "replaces a stale symlink",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(os.Symlink("nowhere", filepath.Join(home, "AGENTS.md")))
				},
			},
			{
				name: "creates the parent directory for a nested link",
				setup: func(t *testing.T, home string) {
					_, err := os.Stat(filepath.Join(home, ".claude"))
					require.New(t).True(os.IsNotExist(err))
				},
			},
			{
				name: "errors when the target is a regular file",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(os.WriteFile(filepath.Join(home, "CLAUDE.md"), []byte("local"), 0o644))
				},
				wantErr: "is not a symlink",
			},
			{
				name: "errors when the target is a regular directory",
				setup: func(t *testing.T, home string) {
					r := require.New(t)
					r.NoError(os.Mkdir(filepath.Join(home, ".claude"), 0o755))
					r.NoError(os.Mkdir(filepath.Join(home, ".claude", "skills"), 0o755))
				},
				wantErr: "is not a symlink",
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				home := t.TempDir()
				if tt.setup != nil {
					tt.setup(t, home)
				}

				err := EnsureSymlinksAt(home)
				if tt.wantErr != "" {
					a.ErrorContains(err, tt.wantErr)
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
	})

	t.Run("HasSymlinksAt", func(t *testing.T) {
		tests := []struct {
			name  string
			setup func(t *testing.T, home string)
			want  bool
		}{
			{
				name: "is false when symlinks are missing",
				want: false,
			},
			{
				name: "is true when every symlink is correct",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(EnsureSymlinksAt(home))
				},
				want: true,
			},
			{
				name: "is false when only some symlinks exist",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(os.Symlink(filepath.Join("scratch", "AGENTS.md"), filepath.Join(home, "AGENTS.md")))
				},
				want: false,
			},
			{
				name: "is false when a symlink points elsewhere",
				setup: func(t *testing.T, home string) {
					r := require.New(t)
					r.NoError(EnsureSymlinksAt(home))
					r.NoError(os.Remove(filepath.Join(home, ".claude", "skills")))
					r.NoError(os.Symlink("elsewhere", filepath.Join(home, ".claude", "skills")))
				},
				want: false,
			},
			{
				name: "is false when a regular file is in the way",
				setup: func(t *testing.T, home string) {
					require.New(t).NoError(os.WriteFile(filepath.Join(home, "AGENTS.md"), []byte("x"), 0o644))
				},
				want: false,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				home := t.TempDir()
				if tt.setup != nil {
					tt.setup(t, home)
				}

				ok, err := HasSymlinksAt(home)
				r.NoError(err)
				a.Equal(tt.want, ok)
			})
		}
	})
}
