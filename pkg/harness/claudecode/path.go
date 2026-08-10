package claudecode

import (
	"bytes"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
)

const goBinPathMarker = "# claude-control: go bin on PATH"

var goBinPathBlock = []byte("\n" + goBinPathMarker + "\n" + `export PATH="$HOME/go/bin:$PATH"` + "\n")

func shellRCFiles() []string {
	return []string{".bashrc", ".profile"}
}

func EnsureGoBinOnPath() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return errors.Wrap(err, "user home dir")
	}
	return EnsureGoBinOnPathAt(home)
}

func EnsureGoBinOnPathAt(home string) error {
	for _, name := range shellRCFiles() {
		if err := ensureGoBinIn(filepath.Join(home, name)); err != nil {
			return err
		}
	}
	return nil
}

func HasGoBinOnPath() (bool, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return false, errors.Wrap(err, "user home dir")
	}
	return HasGoBinOnPathAt(home)
}

func HasGoBinOnPathAt(home string) (bool, error) {
	for _, name := range shellRCFiles() {
		ok, err := rcHasGoBin(filepath.Join(home, name))
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

func ensureGoBinIn(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return errors.Wrapf(err, "read %s", path)
	}
	if bytes.Contains(existing, []byte(goBinPathMarker)) {
		return nil
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return errors.Wrapf(err, "open %s", path)
	}
	defer f.Close()
	if _, err := f.Write(goBinPathBlock); err != nil {
		return errors.Wrapf(err, "write %s", path)
	}
	return nil
}

func rcHasGoBin(path string) (bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.Wrapf(err, "read %s", path)
	}
	return bytes.Contains(data, []byte(goBinPathMarker)), nil
}
