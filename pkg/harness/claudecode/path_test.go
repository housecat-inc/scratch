package claudecode

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnsureGoBinOnPathAt(t *testing.T) {
	tests := []struct {
		name    string
		seed    map[string]string
		runs    int
		wantOK  bool
		wantSub map[string]string
	}{
		{
			name:    "creates rc files from a clean home",
			runs:    1,
			wantOK:  true,
			wantSub: map[string]string{".bashrc": goBinPathMarker, ".profile": goBinPathMarker},
		},
		{
			name:    "appends to existing rc without clobbering",
			seed:    map[string]string{".bashrc": "alias ll='ls -la'\n", ".profile": "umask 022\n"},
			runs:    1,
			wantOK:  true,
			wantSub: map[string]string{".bashrc": "alias ll='ls -la'", ".profile": "umask 022"},
		},
		{
			name:    "idempotent across repeated runs",
			runs:    3,
			wantOK:  true,
			wantSub: map[string]string{".bashrc": goBinPathMarker},
		},
		{
			name:    "no-op when marker already present",
			seed:    map[string]string{".bashrc": "existing\n" + goBinPathMarker + "\nexport PATH=...\n", ".profile": goBinPathMarker + "\n"},
			runs:    1,
			wantOK:  true,
			wantSub: map[string]string{".bashrc": "existing"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			for name, contents := range tc.seed {
				r.NoError(os.WriteFile(filepath.Join(home, name), []byte(contents), 0o644))
			}

			for i := 0; i < tc.runs; i++ {
				r.NoError(EnsureGoBinOnPathAt(home))
			}

			ok, err := HasGoBinOnPathAt(home)
			r.NoError(err)
			a.Equal(tc.wantOK, ok)

			for name, want := range tc.wantSub {
				data, err := os.ReadFile(filepath.Join(home, name))
				r.NoError(err)
				a.Contains(string(data), want)
				a.Equal(1, strings.Count(string(data), goBinPathMarker), "marker should appear once in %s", name)
			}
		})
	}
}

func TestHasGoBinOnPathAt(t *testing.T) {
	tests := []struct {
		name  string
		seed  map[string]string
		want  bool
	}{
		{name: "empty home", want: false},
		{name: "only bashrc has marker", seed: map[string]string{".bashrc": goBinPathMarker}, want: false},
		{name: "only profile has marker", seed: map[string]string{".profile": goBinPathMarker}, want: false},
		{name: "both have marker", seed: map[string]string{".bashrc": goBinPathMarker, ".profile": goBinPathMarker}, want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			home := t.TempDir()
			for name, contents := range tc.seed {
				r.NoError(os.WriteFile(filepath.Join(home, name), []byte(contents), 0o644))
			}

			ok, err := HasGoBinOnPathAt(home)
			r.NoError(err)
			a.Equal(tc.want, ok)
		})
	}
}
