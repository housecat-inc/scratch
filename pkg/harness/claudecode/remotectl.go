package claudecode

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	qrcode "github.com/skip2/go-qrcode"
)

var (
	assistantLineRegex  = regexp.MustCompile(`^\s*●\s+(.+)$`)
	defaultStartTimeout = 20 * time.Second
	nameRegex           = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
	toolCallRegex       = regexp.MustCompile(`^\s*●\s+[A-Z][A-Za-z0-9_]*\(`)
	urlRegex            = regexp.MustCompile(`https://(?:claude\.ai|code\.claude\.com)/code/session_[A-Za-z0-9]+`)
)

type Manager struct {
	ClaudeBin    string
	Hostname     string
	StartTimeout time.Duration
	TmuxBin      string
	TrustProject func(dir string) error

	mu       sync.Mutex
	sessions map[string]*Session
}

type Session struct {
	Dir       string
	ID        string
	Name      string
	Prompt    string
	StartedAt time.Time
	URL       string

	tmuxName string
}

func NewManager() *Manager {
	return &Manager{
		ClaudeBin:    "claude",
		StartTimeout: defaultStartTimeout,
		TmuxBin:      "tmux",
		TrustProject: TrustProject,
		sessions:     make(map[string]*Session),
	}
}

func (m *Manager) Start(name, dir, prompt string) (*Session, error) {
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
	if name != "" && !nameRegex.MatchString(name) {
		return nil, errors.Newf("name %q must match %s", name, nameRegex)
	}

	prefix := sessionPrefix(m.hostname(), name, dir)

	if m.TrustProject != nil {
		if err := m.TrustProject(dir); err != nil {
			return nil, errors.Wrap(err, "trust project")
		}
	}

	id := randomID()
	tmuxName := "claude-" + id
	sanitized := sanitizePrompt(prompt)
	args := []string{"new-session", "-d", "-s", tmuxName, "-e", "CLAUDE_SESSION_PREFIX=" + prefix, "-e", "CLAUDE_SESSION_PROMPT=" + sanitized, "-x", "300", "-y", "50", "-c", dir, m.ClaudeBin, "--remote-control", "--remote-control-session-name-prefix", prefix}
	if prompt != "" {
		args = append(args, prompt)
	}
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
		Name:      prefix,
		Prompt:    sanitized,
		StartedAt: time.Now(),
		URL:       url,
		tmuxName:  tmuxName,
	}
	m.mu.Lock()
	m.sessions[id] = s
	m.mu.Unlock()
	return s, nil
}

func (m *Manager) Recover() error {
	out, err := exec.Command(m.TmuxBin, "list-sessions", "-F", "#{session_name}\t#{session_path}\t#{session_created}").Output()
	if err != nil {
		return nil
	}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 3)
		if len(parts) != 3 || !strings.HasPrefix(parts[0], "claude-") {
			continue
		}
		tmuxName, dir, createdStr := parts[0], parts[1], parts[2]
		id := strings.TrimPrefix(tmuxName, "claude-")

		m.mu.Lock()
		_, exists := m.sessions[id]
		m.mu.Unlock()
		if exists {
			continue
		}

		pane, err := exec.Command(m.TmuxBin, "capture-pane", "-t", tmuxName, "-p", "-J", "-S", "-").Output()
		if err != nil {
			continue
		}
		match := urlRegex.Find(pane)
		if match == nil {
			continue
		}

		prefix := recoverEnv(m.TmuxBin, tmuxName, "CLAUDE_SESSION_PREFIX")
		if prefix == "" {
			prefix = filepath.Base(dir)
		}
		sessionPrompt := recoverEnv(m.TmuxBin, tmuxName, "CLAUDE_SESSION_PROMPT")

		startedAt := time.Now()
		if ts, err := strconv.ParseInt(createdStr, 10, 64); err == nil {
			startedAt = time.Unix(ts, 0)
		}

		m.mu.Lock()
		m.sessions[id] = &Session{
			Dir:       dir,
			ID:        id,
			Name:      prefix,
			Prompt:    sessionPrompt,
			StartedAt: startedAt,
			URL:       string(match),
			tmuxName:  tmuxName,
		}
		m.mu.Unlock()
	}
	return nil
}

func recoverEnv(tmuxBin, tmuxName, key string) string {
	out, err := exec.Command(tmuxBin, "show-environment", "-t", tmuxName, key).Output()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(string(out))
	return strings.TrimPrefix(line, key+"=")
}

func sanitizePrompt(p string) string {
	one := strings.Join(strings.Fields(p), " ")
	if len(one) > 200 {
		one = one[:200] + "…"
	}
	return one
}

func (m *Manager) hostname() string {
	if m.Hostname != "" {
		return m.Hostname
	}
	host, _ := os.Hostname()
	return host
}

func (m *Manager) List() []*Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.After(out[j].StartedAt) })
	return out
}

func (m *Manager) Get(id string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.sessions[id]
}

func (m *Manager) LastMessage(id string) string {
	m.mu.Lock()
	s, ok := m.sessions[id]
	m.mu.Unlock()
	if !ok {
		return ""
	}
	out, err := exec.Command(m.TmuxBin, "capture-pane", "-t", s.tmuxName, "-p", "-J", "-S", "-").Output()
	if err != nil {
		return ""
	}
	return extractLastAssistantMessage(string(out))
}

func extractLastAssistantMessage(pane string) string {
	lines := strings.Split(pane, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		l := lines[i]
		if toolCallRegex.MatchString(l) {
			continue
		}
		m := assistantLineRegex.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		return strings.TrimSpace(m[1])
	}
	return ""
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
		out, err := exec.Command(m.TmuxBin, "capture-pane", "-t", tmuxName, "-p", "-J", "-S", "-").Output()
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

func sessionPrefix(host, name, dir string) string {
	base := name
	if base == "" {
		base = filepath.Base(dir)
		if base == "" || base == "." || base == "/" {
			base = "session"
		}
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
