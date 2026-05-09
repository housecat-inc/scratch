package claudecode

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/cockroachdb/errors"
)

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
	return HasDefaults(filepath.Join(home, ".claude", "settings.json"), settingsDefaults)
}

func Installed() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}
