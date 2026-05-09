package claudecode

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestURLRegex(t *testing.T) {
	tests := []struct {
		in   string
		name string
		want string
	}{
		{name: "claude.ai session URL", in: "open https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht to connect\n", want: "https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht"},
		{name: "code.claude.com session URL", in: "url: https://code.claude.com/code/session_abc123\n", want: "https://code.claude.com/code/session_abc123"},
		{name: "ignores environment redirect URL", in: "Connecting https://claude.ai/code?environment=env_01QB\nthen https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht\n", want: "https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht"},
		{name: "no URL", in: "no link here\n", want: ""},
		{name: "ignores other domains", in: "see https://example.com/code/session_x\n", want: ""},
		{name: "ignores non-session paths", in: "see https://claude.ai/code?environment=env_01QB\n", want: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			got := urlRegex.FindString(tc.in)
			a.Equal(tc.want, got)
		})
	}
}

func TestDefaultName(t *testing.T) {
	a := assert.New(t)
	a.Contains(defaultName("/home/exedev"), "exedev")
	a.Contains(defaultName("/tmp/work/"), "session")
}

func TestManagerStart(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	dir := t.TempDir()
	tmuxBin := writeFakeTmux(t)
	claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_abc123")

	m := NewManager()
	m.TmuxBin = tmuxBin
	m.ClaudeBin = claudeBin
	m.StartTimeout = 3 * time.Second

	s, err := m.Start("my-session", dir)
	r.NoError(err)
	a.Equal("https://claude.ai/code/session_abc123", s.URL)
	a.Equal("my-session", s.Name)
	a.Equal(dir, s.Dir)
	a.NotEmpty(s.ID)

	a.Len(m.List(), 1)
	a.Equal(s, m.Get(s.ID))

	r.NoError(m.Stop(s.ID))
	a.Len(m.List(), 0)
	a.Nil(m.Get(s.ID))

	r.NoError(m.Stop(s.ID), "stop is idempotent")
}

func TestManagerStartErrors(t *testing.T) {
	tmuxBin := writeFakeTmux(t)
	claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_abc")
	dir := t.TempDir()

	tests := []struct {
		dir     string
		name    string
		setup   func(*Manager)
		test    string
		wantErr string
	}{
		{test: "empty dir errors", name: "x", dir: "", wantErr: "dir is required"},
		{test: "missing dir errors", name: "x", dir: "/nope/does/not/exist", wantErr: "stat"},
		{test: "non-dir errors", name: "x", dir: writeFile(t), wantErr: "not a directory"},
		{test: "invalid name errors", name: "bad name!", dir: dir, wantErr: "must match"},
		{
			test: "no URL within timeout errors",
			name: "ok", dir: dir,
			setup: func(m *Manager) {
				m.ClaudeBin = writeFakeClaudeRemoteCtl(t, "")
				m.StartTimeout = 300 * time.Millisecond
			},
			wantErr: "timed out",
		},
	}

	for _, tc := range tests {
		t.Run(tc.test, func(t *testing.T) {
			a := assert.New(t)
			m := NewManager()
			m.TmuxBin = tmuxBin
			m.ClaudeBin = claudeBin
			m.StartTimeout = 2 * time.Second
			if tc.setup != nil {
				tc.setup(m)
			}
			_, err := m.Start(tc.name, tc.dir)
			a.ErrorContains(err, tc.wantErr)
		})
	}
}

func TestManagerStopAll(t *testing.T) {
	r := require.New(t)
	a := assert.New(t)

	dir := t.TempDir()
	m := NewManager()
	m.TmuxBin = writeFakeTmux(t)
	m.ClaudeBin = writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_x")
	m.StartTimeout = 3 * time.Second

	for _, name := range []string{"a", "b", "c"} {
		_, err := m.Start(name, dir)
		r.NoError(err)
	}
	a.Len(m.List(), 3)

	m.StopAll()
	a.Len(m.List(), 0)
}

func TestSessionQRPNG(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)
	s := &Session{URL: "https://claude.ai/code/session_abc"}
	png, err := s.QRPNG(0)
	r.NoError(err)
	a.True(len(png) > 0)
	a.Equal([]byte{0x89, 'P', 'N', 'G'}, png[:4])
}

func writeFile(t *testing.T) string {
	t.Helper()
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "not-a-dir")
	r.NoError(os.WriteFile(path, []byte("x"), 0o644))
	return path
}

func writeFakeTmux(t *testing.T) string {
	t.Helper()
	r := require.New(t)
	panes := t.TempDir()
	t.Setenv("TMUX_PANES", panes)
	path := filepath.Join(t.TempDir(), "tmux")
	r.NoError(os.WriteFile(path, []byte(`#!/bin/sh
SUB=$1
shift
case "$SUB" in
  new-session)
    NAME=
    while [ $# -gt 0 ]; do
      case "$1" in
        -d) shift;;
        -s) NAME=$2; shift 2;;
        -c|-x|-y) shift 2;;
        *) break;;
      esac
    done
    : > "$TMUX_PANES/$NAME"
    ("$@" > "$TMUX_PANES/$NAME" 2>&1) &
    echo $! > "$TMUX_PANES/$NAME.pid"
    ;;
  capture-pane)
    NAME=
    while [ $# -gt 0 ]; do
      case "$1" in
        -t) NAME=$2; shift 2;;
        -p|-J) shift;;
        *) shift;;
      esac
    done
    [ -f "$TMUX_PANES/$NAME" ] && cat "$TMUX_PANES/$NAME"
    ;;
  kill-session)
    NAME=
    while [ $# -gt 0 ]; do
      case "$1" in
        -t) NAME=$2; shift 2;;
        *) shift;;
      esac
    done
    if [ -f "$TMUX_PANES/$NAME.pid" ]; then
      kill $(cat "$TMUX_PANES/$NAME.pid") 2>/dev/null || true
      rm -f "$TMUX_PANES/$NAME.pid"
    fi
    rm -f "$TMUX_PANES/$NAME"
    ;;
esac
`), 0o755))
	return path
}

func writeFakeClaudeRemoteCtl(t *testing.T, url string) string {
	t.Helper()
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "claude")
	body := "#!/bin/sh\n"
	if url != "" {
		body += "echo \"open " + url + " to connect\"\n"
	}
	body += "sleep 5\n"
	r.NoError(os.WriteFile(path, []byte(body), 0o755))
	return path
}
