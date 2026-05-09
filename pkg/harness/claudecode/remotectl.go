package claudecode

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	qrcode "github.com/skip2/go-qrcode"
)

var (
	defaultStartTimeout = 20 * time.Second
	nameRegex           = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	urlRegex            = regexp.MustCompile(`https://(?:claude\.ai|code\.claude\.com)/code/session_[A-Za-z0-9]+`)
)

type Manager struct {
	ClaudeBin    string
	StartTimeout time.Duration
	TmuxBin      string

	mu       sync.Mutex
	sessions map[string]*Session
}

type Session struct {
	Dir       string
	ID        string
	Name      string
	StartedAt time.Time
	URL       string

	tmuxName string
}

func NewManager() *Manager {
	return &Manager{
		ClaudeBin:    "claude",
		StartTimeout: defaultStartTimeout,
		TmuxBin:      "tmux",
		sessions:     make(map[string]*Session),
	}
}

func (m *Manager) Start(name, dir string) (*Session, error) {
	if dir == "" {
		return nil, errors.New("dir is required")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "stat %s", dir)
	}
	if !info.IsDir() {
		return nil, errors.Newf("%s is not a directory", dir)
	}
	if name == "" {
		name = defaultName(dir)
	}
	if !nameRegex.MatchString(name) {
		return nil, errors.Newf("name %q must match %s", name, nameRegex)
	}

	id := randomID()
	tmuxName := "claude-" + id
	args := []string{"new-session", "-d", "-s", tmuxName, "-x", "300", "-y", "50", "-c", dir, m.ClaudeBin, "remote-control", "--name", name}
	if out, err := exec.Command(m.TmuxBin, args...).CombinedOutput(); err != nil {
		return nil, errors.Wrapf(err, "tmux new-session: %s", string(out))
	}

	url, err := m.waitForURL(tmuxName)
	if err != nil {
		_ = exec.Command(m.TmuxBin, "kill-session", "-t", tmuxName).Run()
		return nil, err
	}

	s := &Session{
		Dir:       dir,
		ID:        id,
		Name:      name,
		StartedAt: time.Now(),
		URL:       url,
		tmuxName:  tmuxName,
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(out[j].StartedAt) })
	return out
}

func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

func (m *Manager) Stop(id string) error {
	m.mu.Lock()
	s, ok := m.sessions[id]
	if !ok {
		m.mu.Unlock()
		return nil
	}
	delete(m.sessions, id)
	m.mu.Unlock()

	if out, err := exec.Command(m.TmuxBin, "kill-session", "-t", s.tmuxName).CombinedOutput(); err != nil {
		return errors.Wrapf(err, "tmux kill-session: %s", string(out))
	}
	return nil
}

func (m *Manager) StopAll() {
	m.mu.Lock()
	ids := make([]string, 0, len(m.sessions))
	for id := range m.sessions {
		ids = append(ids, id)
	}
	m.mu.Unlock()
	for _, id := range ids {
		_ = m.Stop(id)
	}
}

func (m *Manager) waitForURL(tmuxName string) (string, error) {
	timeout := m.StartTimeout
	if timeout == 0 {
		timeout = defaultStartTimeout
	}
	deadline := time.Now().Add(timeout)
	var lastOut []byte
	for time.Now().Before(deadline) {
		out, err := exec.Command(m.TmuxBin, "capture-pane", "-t", tmuxName, "-p", "-J").Output()
		if err == nil {
			lastOut = out
			if match := urlRegex.Find(out); match != nil {
				return string(match), nil
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	return "", errors.Newf("timed out waiting for remote-control URL; pane: %s", tail(string(lastOut), 400))
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

func (s *Session) QRPNG(size int) ([]byte, error) {
	if size <= 0 {
		size = 256
	}
	return qrcode.Encode(s.URL, qrcode.Medium, size)
}

func defaultName(dir string) string {
	host, _ := os.Hostname()
	base := dir
	for i := len(dir) - 1; i >= 0; i-- {
		if dir[i] == '/' {
			base = dir[i+1:]
			break
		}
	}
	if base == "" {
		base = "session"
	}
	if host == "" {
		return base
	}
	return host + "-" + base
}

func randomID() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
