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

func TestLogin(t *testing.T) {
	tests := []struct {
		code    string
		name    string
		wantErr bool
	}{
		{name: "valid code returns nil", code: "good"},
		{name: "invalid code errors", code: "bad", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			script := writeFakeClaude(t)
			l, err := StartLoginCmd(exec.Command("sh", script))
			r.NoError(err)
			t.Cleanup(func() { _ = l.Close() })

			a.Contains(l.URL(), "https://claude.com/")

			err = l.SubmitCode(tc.code)
			if tc.wantErr {
				a.Error(err)
				return
			}
			a.NoError(err)
		})
	}
}

func TestStartLoginNoURLTimesOut(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	script := filepath.Join(t.TempDir(), "fake-no-url.sh")
	r.NoError(os.WriteFile(script, []byte("#!/bin/sh\nsleep 5\n"), 0o755))

	orig := loginURLTimeout
	loginURLTimeout = 200 * time.Millisecond
	t.Cleanup(func() { loginURLTimeout = orig })

	_, err := StartLoginCmd(exec.Command("sh", script))
	a.Error(err)
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
