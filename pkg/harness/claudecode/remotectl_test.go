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
		{name: "matches a claude.ai session URL", in: "open https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht to connect\n", want: "https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht"},
		{name: "matches a code.claude.com session URL", in: "url: https://code.claude.com/code/session_abc123\n", want: "https://code.claude.com/code/session_abc123"},
		{name: "skips an environment redirect URL", in: "Connecting https://claude.ai/code?environment=env_01QB\nthen https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht\n", want: "https://claude.ai/code/session_01HcAF1wLDAtAuisjuGPAdht"},
		{name: "finds nothing without a URL", in: "no link here\n", want: ""},
		{name: "ignores other domains", in: "see https://example.com/code/session_x\n", want: ""},
		{name: "ignores non-session paths", in: "see https://claude.ai/code?environment=env_01QB\n", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			got := urlRegex.FindString(tt.in)
			a.Equal(tt.want, got)
		})
	}
}

func TestExtractLastAssistantMessage(t *testing.T) {
	tests := []struct {
		name string
		pane string
		want string
	}{
		{
			name: "returns a plain assistant message",
			pane: "● Hello world\n",
			want: "Hello world",
		},
		{
			name: "skips a tool call before the assistant reply",
			pane: "● Bash(go test ./...)\n  ⎿  ok pkg/foo\n● All tests pass on main.\n",
			want: "All tests pass on main.",
		},
		{
			name: "skips a trailing tool call to find the prior reply",
			pane: "● Earlier reply from claude.\n● Read(/etc/hosts)\n  ⎿  127.0.0.1 localhost\n",
			want: "Earlier reply from claude.",
		},
		{
			name: "ignores TUI chrome",
			pane: "──────────\n❯ user typing\n  ⏵⏵ bypass permissions on (shift+tab to cycle)                Remote Control active\n",
			want: "",
		},
		{
			name: "treats a nested-parens tool call as a tool call",
			pane: "● Edit(pkg/server/code/code.go)\n  ⎿  Modified 1 line\n",
			want: "",
		},
		{
			name: "treats a lowercase word with parens as prose",
			pane: "● go test (1 failure)\n",
			want: "go test (1 failure)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, extractLastAssistantMessage(tt.pane))
		})
	}
}

func TestSessionPrefix(t *testing.T) {
	tests := []struct {
		dir  string
		host string
		name string
		user string
		want string
	}{
		{name: "joins the host and user name", host: "jukelab", user: "foobar", dir: "/tmp/x", want: "jukelab-foobar"},
		{name: "joins the host and dir basename without a name", host: "jukelab", dir: "/home/exedev", want: "jukelab-exedev"},
		{name: "uses the basename before a trailing slash", host: "h", dir: "/tmp/work/", want: "h-work"},
		{name: "falls back to session for the root dir", host: "h", dir: "/", want: "h-session"},
		{name: "returns the base alone without a host", user: "n", dir: "/tmp/x", want: "n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.want, sessionPrefix(tt.host, tt.user, tt.dir))
		})
	}
}

func TestStart(t *testing.T) {
	t.Run("starts lists and stops a session", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		tmuxBin := writeFakeTmux(t)
		claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_abc123")

		var trustedDir string
		m := NewManager()
		m.ClaudeBin = claudeBin
		m.Hostname = "jukelab"
		m.StartTimeout = 3 * time.Second
		m.TmuxBin = tmuxBin
		m.TrustProject = func(d string) error { trustedDir = d; return nil }

		s, err := m.Start("foobar", dir, "")
		r.NoError(err)
		a.Equal("https://claude.ai/code/session_abc123", s.URL)
		a.Equal("jukelab-foobar", s.Name)
		a.Equal(dir, s.Dir)
		a.Equal(dir, trustedDir)
		a.NotEmpty(s.ID)

		a.Len(m.List(), 1)
		a.Equal(s, m.Get(s.ID))

		r.NoError(m.Stop(s.ID))
		a.Len(m.List(), 0)
		a.Nil(m.Get(s.ID))

		r.NoError(m.Stop(s.ID), "stop is idempotent")
	})

	tmuxBin := writeFakeTmux(t)
	claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_abc")
	dir := t.TempDir()

	tests := []struct {
		dir     string
		name    string
		session string
		setup   func(*Manager)
		wantErr string
	}{
		{name: "rejects an empty dir", session: "x", dir: "", wantErr: "dir is required"},
		{name: "rejects a missing dir", session: "x", dir: "/nope/does/not/exist", wantErr: "stat"},
		{name: "rejects a dir that is a file", session: "x", dir: writeFile(t), wantErr: "not a directory"},
		{name: "rejects an invalid name", session: "bad name!", dir: dir, wantErr: "must match"},
		{
			name:    "times out when no URL is printed",
			session: "ok", dir: dir,
			setup: func(m *Manager) {
				m.ClaudeBin = writeFakeClaudeRemoteCtl(t, "")
				m.StartTimeout = 300 * time.Millisecond
			},
			wantErr: "timed out",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			m := NewManager()
			m.TmuxBin = tmuxBin
			m.ClaudeBin = claudeBin
			m.StartTimeout = 2 * time.Second
			m.TrustProject = func(string) error { return nil }
			if tt.setup != nil {
				tt.setup(m)
			}
			_, err := m.Start(tt.session, tt.dir, "")
			a.ErrorContains(err, tt.wantErr)
		})
	}
}

func TestRecover(t *testing.T) {
	t.Run("restores sessions from tmux in a fresh manager", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		tmuxBin := writeFakeTmux(t)
		claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_recovered")

		first := NewManager()
		first.ClaudeBin = claudeBin
		first.Hostname = "jukelab"
		first.StartTimeout = 3 * time.Second
		first.TmuxBin = tmuxBin
		first.TrustProject = func(string) error { return nil }

		s, err := first.Start("foobar", dir, "")
		r.NoError(err)

		fresh := NewManager()
		fresh.TmuxBin = tmuxBin
		r.NoError(fresh.Recover())
		a.Len(fresh.List(), 1)

		recovered := fresh.Get(s.ID)
		r.NotNil(recovered, "session id survives across managers via tmux scan")
		a.Equal(s.URL, recovered.URL)
		a.Equal(s.Dir, recovered.Dir)
		a.Equal(s.Name, recovered.Name, "prefix recovered from tmux env")
	})

	t.Run("does not duplicate sessions when run twice", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		tmuxBin := writeFakeTmux(t)
		claudeBin := writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_x")

		m := NewManager()
		m.ClaudeBin = claudeBin
		m.Hostname = "h"
		m.StartTimeout = 3 * time.Second
		m.TmuxBin = tmuxBin
		m.TrustProject = func(string) error { return nil }

		_, err := m.Start("alpha", dir, "")
		r.NoError(err)

		r.NoError(m.Recover())
		r.NoError(m.Recover())
		a.Len(m.List(), 1, "running Recover twice does not duplicate sessions")
	})

	t.Run("skips sessions that never printed a URL", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		tmuxBin := writeFakeTmux(t)
		claudeBin := writeFakeClaudeRemoteCtl(t, "")

		starter := NewManager()
		starter.ClaudeBin = claudeBin
		starter.Hostname = "h"
		starter.StartTimeout = 300 * time.Millisecond
		starter.TmuxBin = tmuxBin
		starter.TrustProject = func(string) error { return nil }

		_, err := starter.Start("nourl", dir, "")
		a.Error(err, "session start fails because no URL emitted")

		fresh := NewManager()
		fresh.TmuxBin = tmuxBin
		r.NoError(fresh.Recover())
		a.Empty(fresh.List(), "session without URL is skipped on recover")
	})
}

func TestStopAll(t *testing.T) {
	t.Run("stops every session", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		dir := t.TempDir()
		m := NewManager()
		m.TmuxBin = writeFakeTmux(t)
		m.ClaudeBin = writeFakeClaudeRemoteCtl(t, "https://claude.ai/code/session_x")
		m.StartTimeout = 3 * time.Second
		m.TrustProject = func(string) error { return nil }

		for _, name := range []string{"a", "b", "c"} {
			_, err := m.Start(name, dir, "")
			r.NoError(err)
		}
		a.Len(m.List(), 3)

		m.StopAll()
		a.Len(m.List(), 0)
	})
}

func TestQRPNG(t *testing.T) {
	t.Run("encodes the session URL as a PNG", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)
		s := &Session{URL: "https://claude.ai/code/session_abc"}
		png, err := s.QRPNG(0)
		r.NoError(err)
		a.True(len(png) > 0)
		a.Equal([]byte{0x89, 'P', 'N', 'G'}, png[:4])
	})
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
    DIR=
    ENV_PAIRS=
    while [ $# -gt 0 ]; do
      case "$1" in
        -d) shift;;
        -s) NAME=$2; shift 2;;
        -c) DIR=$2; shift 2;;
        -e) ENV_PAIRS="$ENV_PAIRS$2|"; shift 2;;
        -x|-y) shift 2;;
        *) break;;
      esac
    done
    : > "$TMUX_PANES/$NAME"
    echo "$DIR" > "$TMUX_PANES/$NAME.dir"
    date +%s > "$TMUX_PANES/$NAME.created"
    : > "$TMUX_PANES/$NAME.env"
    : > "$TMUX_PANES/$NAME.keys"
    IFS='|'
    for pair in $ENV_PAIRS; do
      echo "$pair" >> "$TMUX_PANES/$NAME.env"
    done
    unset IFS
    export RC_KEYS="$TMUX_PANES/$NAME.keys"
    ("$@" > "$TMUX_PANES/$NAME" 2>&1) &
    echo $! > "$TMUX_PANES/$NAME.pid"
    ;;
  send-keys)
    NAME=
    while [ $# -gt 0 ]; do
      case "$1" in
        -t) NAME=$2; shift 2;;
        -l) shift;;
        *) printf '%s' "$1" >> "$TMUX_PANES/$NAME.keys"; shift;;
      esac
    done
    ;;
  list-sessions)
    for f in "$TMUX_PANES"/*.dir; do
      [ -e "$f" ] || continue
      NAME=$(basename "$f" .dir)
      DIR=$(cat "$f")
      CREATED=$(cat "$TMUX_PANES/$NAME.created" 2>/dev/null)
      printf '%s\t%s\t%s\n' "$NAME" "$DIR" "$CREATED"
    done
    ;;
  show-environment)
    NAME=
    KEY=
    while [ $# -gt 0 ]; do
      case "$1" in
        -t) NAME=$2; shift 2;;
        *) KEY=$1; shift;;
      esac
    done
    if [ -f "$TMUX_PANES/$NAME.env" ]; then
      grep "^$KEY=" "$TMUX_PANES/$NAME.env"
    fi
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
    rm -f "$TMUX_PANES/$NAME" "$TMUX_PANES/$NAME.dir" "$TMUX_PANES/$NAME.created" "$TMUX_PANES/$NAME.env" "$TMUX_PANES/$NAME.keys"
    ;;
esac
`), 0o755))
	return path
}

// writeFakeClaudeRemoteCtl models Claude Code >= 2.1.168: it reports remote
// control as active but only prints the session URL once the /remote-control
// menu is opened (simulated via keys written to RC_KEYS).
func writeFakeClaudeRemoteCtl(t *testing.T, url string) string {
	t.Helper()
	r := require.New(t)
	path := filepath.Join(t.TempDir(), "claude")
	body := "#!/bin/sh\n"
	body += "echo 'Remote Control active'\n"
	if url != "" {
		body += "n=0\n"
		body += "while [ $n -lt 100 ]; do\n"
		body += "  if [ -n \"$RC_KEYS\" ] && grep -q 'remote-control' \"$RC_KEYS\" 2>/dev/null; then\n"
		body += "    echo 'This session is available in the Claude mobile app and at " + url + ".'\n"
		body += "    break\n"
		body += "  fi\n"
		body += "  sleep 0.1\n"
		body += "  n=$((n+1))\n"
		body += "done\n"
	}
	body += "sleep 5\n"
	r.NoError(os.WriteFile(path, []byte(body), 0o755))
	return path
}
