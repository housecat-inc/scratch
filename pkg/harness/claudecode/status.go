package claudecode

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
)

func Authenticated() (bool, error) {
	return authenticated(exec.Command("claude", "auth", "status", "--json"))
}

func authenticated(cmd *exec.Cmd) (bool, error) {
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	var status struct {
		LoggedIn bool `json:"loggedIn"`
	}
	if err := json.Unmarshal(out, &status); err != nil {
		return false, errors.Wrap(err, "parse auth status")
	}
	return status.LoggedIn, nil
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
