package claudecode

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
)

const npmLatestURL = "https://registry.npmjs.org/@anthropic-ai/claude-code/latest"

type Credentials struct {
	ClaudeAIOauth ClaudeAIOauth `json:"claudeAiOauth"`
}

type ClaudeAIOauth struct {
	AccessToken  string `json:"accessToken"`
	ExpiresAt    int64  `json:"expiresAt"`
	RefreshToken string `json:"refreshToken"`
}

func Authenticated() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.Wrap(err, "user home dir")
	}
	return AuthenticatedAt(filepath.Join(home, ".claude", ".credentials.json"))
}

func AuthenticatedAt(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "read %s", path)
	}
	if len(data) == 0 {
		return false, nil
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return false, errors.Wrapf(err, "parse %s", path)
	}
	if c.ClaudeAIOauth.AccessToken == "" {
		return false, nil
	}
	return time.Now().UnixMilli() < c.ClaudeAIOauth.ExpiresAt, nil
}

func Configured() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.Wrap(err, "user home dir")
	}
	return ConfiguredAt(home)
}

func ConfiguredAt(home string) (bool, error) {
	ok, err := HasDefaults(filepath.Join(home, ".claude.json"), claudeJSONDefaults)
	if err != nil || !ok {
		return ok, err
	}
	if ok, err := HasDefaults(filepath.Join(home, ".claude", "settings.json"), settingsDefaults); err != nil || !ok {
		return ok, err
	}
	if ok, err := agents.InstalledAt(filepath.Join(home, "scratch")); err != nil || !ok {
		return ok, err
	}
	if ok, err := HasSymlinksAt(home); err != nil || !ok {
		return ok, err
	}
	return HasGoBinOnPathAt(home)
}

func Installed() bool {
	for _, bin := range []string{"claude", "tmux"} {
		if _, err := exec.LookPath(bin); err != nil {
			return false
		}
	}
	return true
}

func Version() (string, error) {
	if _, err := exec.LookPath("claude"); err != nil {
		return "", nil
	}
	out, err := exec.Command("claude", "--version").Output()
	if err != nil {
		return "", errors.Wrap(err, "claude --version")
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[0], nil
}

func LatestVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, npmLatestURL, nil)
	if err != nil {
		return "", errors.Wrap(err, "build npm registry request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", errors.Wrap(err, "fetch npm registry")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", errors.Newf("npm registry returned %s", resp.Status)
	}
	var body struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", errors.Wrap(err, "decode npm response")
	}
	return body.Version, nil
}

func UpdateAvailable() (bool, error) {
	current, err := Version()
	if err != nil || current == "" {
		return false, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	latest, err := LatestVersion(ctx)
	if err != nil || latest == "" {
		return false, err
	}
	return current != latest, nil
}
