package codexcode

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

func Installed() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func Authenticated() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.Wrap(err, "user home dir")
	}
	return AuthenticatedAt(home)
}

func AuthenticatedAt(home string) (bool, error) {
	if _, err := os.Stat(filepath.Join(home, ".codex", "auth.json")); err == nil {
		return true, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return false, errors.Wrap(err, "stat codex auth")
	}
	return false, nil
}

func Version() (string, error) {
	if _, err := exec.LookPath("codex"); err != nil {
		return "", nil
	}
	return version(exec.Command("codex", "--version"))
}

func version(cmd *exec.Cmd) (string, error) {
	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "codex --version")
	}
	fields := strings.Fields(string(out))
	if len(fields) == 0 {
		return "", nil
	}
	return fields[len(fields)-1], nil
}
