package claudecode

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/agents"
)

const installScriptURL = "https://claude.ai/install.sh"

type ClaudeConfig struct {
	HasCompletedOnboarding bool               `json:"hasCompletedOnboarding"`
	HasUsedRemoteControl   bool               `json:"hasUsedRemoteControl"`
	Projects               map[string]Project `json:"projects,omitempty"`
	RemoteDialogSeen       bool               `json:"remoteDialogSeen"`
}

type Permissions struct {
	DefaultMode string `json:"defaultMode"`
}

type Project struct {
	HasTrustDialogAccepted bool `json:"hasTrustDialogAccepted"`
}

type Settings struct {
	Permissions                       Permissions `json:"permissions"`
	SkipDangerousModePermissionPrompt bool        `json:"skipDangerousModePermissionPrompt"`
	Theme                             string      `json:"theme"`
}

var claudeJSONDefaults = ClaudeConfig{
	HasCompletedOnboarding: true,
	HasUsedRemoteControl:   true,
	RemoteDialogSeen:       true,
}

var settingsDefaults = Settings{
	Permissions:                       Permissions{DefaultMode: "bypassPermissions"},
	SkipDangerousModePermissionPrompt: true,
	Theme:                             "auto",
}

func EnsureInstalled() error {
	if err := EnsureTmuxInstalled(); err != nil {
		return err
	}
	if _, err := exec.LookPath("claude"); err == nil {
		return run("claude", "update")
	}
	return run("sh", "-c", "curl -fsSL "+installScriptURL+" | bash")
}

func EnsureTmuxInstalled() error {
	if _, err := exec.LookPath("tmux"); err == nil {
		return nil
	}
	args, err := tmuxInstallCommand()
	if err != nil {
		return err
	}
	return run(args[0], args[1:]...)
}

func tmuxInstallCommand() ([]string, error) {
	switch runtime.GOOS {
	case "darwin":
		if _, err := exec.LookPath("brew"); err != nil {
			return nil, errors.New("install tmux: brew not found")
		}
		return []string{"brew", "install", "tmux"}, nil
	case "linux":
		managers := [][]string{
			{"apk", "add", "tmux"},
			{"apt-get", "install", "-y", "tmux"},
			{"dnf", "install", "-y", "tmux"},
			{"pacman", "-S", "--noconfirm", "tmux"},
			{"yum", "install", "-y", "tmux"},
		}
		for _, args := range managers {
			if _, err := exec.LookPath(args[0]); err != nil {
				continue
			}
			if os.Geteuid() != 0 {
				args = append([]string{"sudo", "-n"}, args...)
			}
			return args, nil
		}
		return nil, errors.New("install tmux: no supported package manager found")
	}
	return nil, errors.Newf("install tmux: unsupported OS %s", runtime.GOOS)
}

func HasDefaults(path string, defaults any) (bool, error) {
	defaultsMap, err := toMap(defaults)
	if err != nil {
		return false, errors.Wrap(err, "encode defaults")
	}
	current, err := readJSONObject(path)
	if err != nil {
		return false, err
	}
	return !mergeMissing(current, defaultsMap), nil
}

func MergeDefaults(path string, defaults any) error {
	defaultsMap, err := toMap(defaults)
	if err != nil {
		return errors.Wrap(err, "encode defaults")
	}

	current, err := readJSONObject(path)
	if err != nil {
		return err
	}

	if !mergeMissing(current, defaultsMap) {
		return nil
	}

	return writeJSONObject(path, current)
}

func Configure() error {
	if err := WriteDefaults(); err != nil {
		return err
	}
	if err := agents.InstallOrUpdate(); err != nil {
		return err
	}
	if err := EnsureSymlinks(); err != nil {
		return err
	}
	return EnsureGoBinOnPath()
}

func Setup() error {
	if err := EnsureInstalled(); err != nil {
		return err
	}
	return Configure()
}

func WriteDefaults() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "user home dir")
	}
	if err := MergeDefaults(filepath.Join(home, ".claude.json"), claudeJSONDefaults); err != nil {
		return err
	}
	return MergeDefaults(filepath.Join(home, ".claude", "settings.json"), settingsDefaults)
}

func TrustProject(dir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "user home dir")
	}
	return TrustProjectAt(filepath.Join(home, ".claude.json"), dir)
}

func TrustProjectAt(path, dir string) error {
	return MergeDefaults(path, map[string]any{
		"projects": map[string]any{
			dir: map[string]any{"hasTrustDialogAccepted": true},
		},
	})
}

func mergeMissing(dst, defaults map[string]any) bool {
	changed := false
	for k, dv := range defaults {
		cv, ok := dst[k]
		if !ok {
			dst[k] = dv
			changed = true
			continue
		}
		dvMap, dvIsMap := dv.(map[string]any)
		cvMap, cvIsMap := cv.(map[string]any)
		if dvIsMap && cvIsMap && mergeMissing(cvMap, dvMap) {
			changed = true
		}
	}
	return changed
}

func readJSONObject(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return map[string]any{}, nil
	}
	if err != nil {
		return nil, errors.Wrapf(err, "read %s", path)
	}
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, errors.Wrapf(err, "parse %s", path)
	}
	return out, nil
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return errors.Wrapf(err, "run %s", name)
	}
	return nil
}

func toMap(v any) (map[string]any, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	out := map[string]any{}
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func writeJSONObject(path string, v map[string]any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return errors.Wrapf(err, "mkdir %s", filepath.Dir(path))
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return errors.Wrap(err, "encode json")
	}
	out = append(out, '\n')
	if err := os.WriteFile(path, out, 0o644); err != nil {
		return errors.Wrapf(err, "write %s", path)
	}
	return nil
}
