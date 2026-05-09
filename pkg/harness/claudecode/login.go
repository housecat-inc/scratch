package claudecode

import (
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/creack/pty"
)

var (
	loginCodeRe      = regexp.MustCompile(`Login successful|Invalid code`)
	loginCodeTimeout = 30 * time.Second
	loginURLRe       = regexp.MustCompile(`https://claude\.com[^\s]+`)
	loginURLTimeout  = 30 * time.Second
)

type Login struct {
	buf strings.Builder
	cmd *exec.Cmd
	mu  sync.Mutex
	pty io.ReadWriteCloser
	url string
}

func StartLogin() (*Login, error) {
	return StartLoginCmd(exec.Command("claude", "auth", "login", "--claudeai"))
}

func StartLoginCmd(cmd *exec.Cmd) (*Login, error) {
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "pty start")
	}
	l := &Login{cmd: cmd, pty: f}
	go l.readLoop()

	url, err := l.waitFor(loginURLRe, 0, loginURLTimeout)
	if err != nil {
		_ = l.Close()
		return nil, errors.Wrap(err, "wait for login url")
	}
	l.url = url
	return l, nil
}

func (l *Login) Close() error {
	if l.cmd != nil && l.cmd.Process != nil {
		_ = l.cmd.Process.Kill()
	}
	if l.pty != nil {
		_ = l.pty.Close()
	}
	if l.cmd != nil {
		_ = l.cmd.Wait()
	}
	return nil
}

func (l *Login) Output() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

func (l *Login) SubmitCode(code string) error {
	l.mu.Lock()
	from := l.buf.Len()
	l.mu.Unlock()

	if _, err := l.pty.Write([]byte(code + "\n")); err != nil {
		return errors.Wrap(err, "write code")
	}

	match, err := l.waitFor(loginCodeRe, from, loginCodeTimeout)
	if err != nil {
		return errors.Wrap(err, "wait for login result")
	}
	if match == "Invalid code" {
		return errors.New("invalid code")
	}
	return nil
}

func (l *Login) URL() string {
	return l.url
}

func (l *Login) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := l.pty.Read(buf)
		if n > 0 {
			l.mu.Lock()
			l.buf.Write(buf[:n])
			l.mu.Unlock()
		}
		if err != nil {
			return
		}
	}
}

func (l *Login) waitFor(re *regexp.Regexp, from int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		s := l.buf.String()
		l.mu.Unlock()
		if from < len(s) {
			if m := re.FindString(s[from:]); m != "" {
				return m, nil
			}
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", errors.New("timeout")
}
