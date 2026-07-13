package codexcode

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
	ansiRe          = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")
	deviceCodeRe    = regexp.MustCompile(`\b[A-Z0-9]{4}-[A-Z0-9]{4,6}\b`)
	deviceURLRe     = regexp.MustCompile(`https://auth\.openai\.com/codex/device[^\s]*`)
	loginURLTimeout = 30 * time.Second
)

type DeviceLogin struct {
	buf  strings.Builder
	code string
	cmd  *exec.Cmd
	done bool
	err  error
	mu   sync.Mutex
	pty  io.ReadWriteCloser
	url  string
}

func StartDeviceLogin() (*DeviceLogin, error) {
	return StartDeviceLoginCmd(exec.Command("codex", "login", "--device-auth"))
}

func StartDeviceLoginCmd(cmd *exec.Cmd) (*DeviceLogin, error) {
	f, err := pty.Start(cmd)
	if err != nil {
		return nil, errors.Wrap(err, "pty start")
	}
	l := &DeviceLogin{cmd: cmd, pty: f}
	go l.readLoop()
	go l.waitLoop()

	url, err := l.waitFor(deviceURLRe, loginURLTimeout)
	if err != nil {
		_ = l.Close()
		return nil, errors.Wrap(err, "wait for device url")
	}
	code, err := l.waitFor(deviceCodeRe, loginURLTimeout)
	if err != nil {
		_ = l.Close()
		return nil, errors.Wrap(err, "wait for device code")
	}

	l.mu.Lock()
	l.url, l.code = url, code
	l.mu.Unlock()
	return l, nil
}

func (l *DeviceLogin) Close() error {
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

func (l *DeviceLogin) Code() string { return l.code }

func (l *DeviceLogin) URL() string { return l.url }

func (l *DeviceLogin) Done() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.done
}

func (l *DeviceLogin) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *DeviceLogin) readLoop() {
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

func (l *DeviceLogin) waitLoop() {
	err := l.cmd.Wait()
	l.mu.Lock()
	l.done = true
	l.err = err
	l.mu.Unlock()
}

func (l *DeviceLogin) waitFor(re *regexp.Regexp, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		l.mu.Lock()
		s := ansiRe.ReplaceAllString(l.buf.String(), "")
		done := l.done
		l.mu.Unlock()
		if m := re.FindString(s); m != "" {
			return m, nil
		}
		if done {
			return "", errors.New("process exited before match")
		}
		time.Sleep(50 * time.Millisecond)
	}
	return "", errors.New("timeout")
}
