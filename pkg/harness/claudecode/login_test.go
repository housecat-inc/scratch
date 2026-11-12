package claudecode

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartLoginCmd(t *testing.T) {
	t.Run("times out when no login URL is printed", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		script := filepath.Join(t.TempDir(), "fake-no-url.sh")
		r.NoError(os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755))

		orig := loginURLTimeout
		loginURLTimeout = 200 * time.Millisecond
		t.Cleanup(func() { loginURLTimeout = orig })

		_, err := StartLoginCmd(exec.Command("sh", script))
		a.Error(err)
	})
}

func TestSubmitCode(t *testing.T) {
	tests := []struct {
		code    string
		name    string
		wantErr bool
	}{
		{name: "accepts a valid code", code: "good"},
		{name: "rejects an invalid code", code: "bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			script := writeFakeClaude(t)
			l, err := StartLoginCmd(exec.Command("sh", script))
			r.NoError(err)
			t.Cleanup(func() { _ = l.Close() })

			a.Contains(l.URL(), "https://claude.com/")

			err = l.SubmitCode(tt.code)
			if tt.wantErr {
				a.Error(err)
				return
			}
			a.NoError(err)
		})
	}

	t.Run("succeeds when the browser already completed login", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		path := filepath.Join(t.TempDir(), "fake-claude-browser.sh")
		r.NoError(os.WriteFile(path, []byte(`#!/bin/sh
echo "Visit https://claude.com/auth?code=abc to log in"
echo "Login successful"
`), 0o755))

		l, err := StartLoginCmd(exec.Command("sh", path))
		r.NoError(err)
		t.Cleanup(func() { _ = l.Close() })

		a.NoError(l.SubmitCode("unused"))
	})
}

func writeFakeClaude(t *testing.T) string {
	t.Helper()
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "fake-claude.sh")
	r.NoError(os.WriteFile(path, []byte(`#!/bin/sh
echo "Visit https://claude.com/auth?code=abc to log in"
read line
if [ "$line" = "good" ]; then
  echo "Login successful"
else
  echo "Invalid code"
fi
`), 0o755))
	return path
}
