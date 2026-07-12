package codexcode

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStartDeviceLogin(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "fake-codex.sh")
	r.NoError(os.WriteFile(path, []byte("#!/bin/sh\n"+
		"printf 'Open this link\\n  \\033[94mhttps://auth.openai.com/codex/device\\033[0m\\n'\n"+
		"printf 'Enter this one-time code\\n  \\033[94m32LO-RHYUX\\033[0m\\n'\n"), 0o755))

	l, err := StartDeviceLoginCmd(exec.Command("sh", path))
	r.NoError(err)
	t.Cleanup(func() { _ = l.Close() })

	a.Contains(l.URL(), "https://auth.openai.com/codex/device")
	a.Equal("32LO-RHYUX", l.Code())

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !l.Done() {
		time.Sleep(20 * time.Millisecond)
	}
	a.True(l.Done())
	a.NoError(l.Err())
}

func TestStartDeviceLoginNoURLTimesOut(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	path := filepath.Join(t.TempDir(), "fake-no-url.sh")
	r.NoError(os.WriteFile(path, []byte("#!/bin/sh\nsleep 5\n"), 0o755))

	orig := loginURLTimeout
	loginURLTimeout = 200 * time.Millisecond
	t.Cleanup(func() { loginURLTimeout = orig })

	_, err := StartDeviceLoginCmd(exec.Command("sh", path))
	a.Error(err)
}
