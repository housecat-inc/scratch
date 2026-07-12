package codexcode

import (
	"os/exec"
	"strings"

	"github.com/cockroachdb/errors"
)

func Installed() bool {
	_, err := exec.LookPath("codex")
	return err == nil
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
