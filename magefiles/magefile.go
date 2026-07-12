//go:build mage

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

const (
	appDevPort = "8000"
	pkg        = "./cmd/scratch"
	service    = "scratch.service"
)

func bin() string { return filepath.Join(os.Getenv("HOME"), "go", "bin", "scratch") }

func port() string {
	if p := os.Getenv("PORT"); p != "" {
		return p
	}
	return "8888"
}

func unitDir() string {
	return filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
}

func unitPath() string { return filepath.Join(unitDir(), service) }

func Build() error {
	mg.Deps(Generate)
	return sh.RunV("go", "build", "-o", bin(), pkg)
}

func Dev() {
	mg.SerialDeps(cleanDevAppPort)
	mg.Deps(devApp, devScratch)
}

func cleanDevAppPort() error {
	pids, err := listenPIDs(appDevPort)
	if err != nil {
		return err
	}
	if len(pids) == 0 {
		return nil
	}
	root, err := os.Getwd()
	if err != nil {
		return errors.Wrap(err, "get working directory")
	}
	expected := filepath.Join(root, "tmp", "app", "app")
	for _, pid := range pids {
		ppid, command, err := processInfo(pid)
		if err != nil {
			return err
		}
		if ppid != 1 || !strings.Contains(command, expected) {
			return errors.Errorf("port %s is in use by pid %d (%s); not killing because it is not an orphaned %s", appDevPort, pid, command, expected)
		}
		fmt.Printf("killing orphaned app dev server pid %d on port %s\n", pid, appDevPort)
		if err := killProcess(pid); err != nil {
			return err
		}
	}
	return nil
}

func devApp() error {
	return sh.RunV("go", "tool", "air", "-c", "cmd/app/.air.toml")
}

func devScratch() error {
	return sh.RunV("go", "tool", "air", "-c", "cmd/scratch/.air.toml")
}

func killProcess(pid int) error {
	proc, err := os.FindProcess(pid)
	if err != nil {
		return errors.Wrapf(err, "find process %d", pid)
	}
	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return errors.Wrapf(err, "terminate process %d", pid)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(pid) {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := proc.Signal(syscall.SIGKILL); err != nil {
		return errors.Wrapf(err, "kill process %d", pid)
	}
	return nil
}

func listenPIDs(port string) ([]int, error) {
	cmd := exec.Command("lsof", "-tiTCP:"+port, "-sTCP:LISTEN")
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); ok && len(bytes.TrimSpace(exit.Stderr)) == 0 && len(bytes.TrimSpace(out)) == 0 {
			return nil, nil
		}
		return nil, errors.Wrapf(err, "list listeners on port %s", port)
	}
	lines := strings.Fields(string(out))
	pids := make([]int, 0, len(lines))
	for _, line := range lines {
		pid, err := strconv.Atoi(line)
		if err != nil {
			return nil, errors.Wrapf(err, "parse pid %q", line)
		}
		pids = append(pids, pid)
	}
	return pids, nil
}

func processExists(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func processInfo(pid int) (int, string, error) {
	out, err := exec.Command("ps", "-p", strconv.Itoa(pid), "-o", "ppid=", "-o", "command=").Output()
	if err != nil {
		return 0, "", errors.Wrapf(err, "inspect process %d", pid)
	}
	line := strings.TrimSpace(string(out))
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0, "", errors.Errorf("unexpected process info for pid %d: %q", pid, line)
	}
	ppid, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", errors.Wrapf(err, "parse parent pid for %d", pid)
	}
	return ppid, strings.TrimSpace(strings.TrimPrefix(line, parts[0])), nil
}

func Generate() error {
	if err := sh.RunV("go", "generate", "./pkg/db/...", "./pkg/ui/..."); err != nil {
		return err
	}
	return sh.RunV("go", "generate", "./pkg/api/...")
}

func Install() error {
	mg.Deps(Build)
	return nil
}

func Service() error {
	if err := os.MkdirAll(unitDir(), 0o755); err != nil {
		return err
	}
	unit := fmt.Sprintf(`[Unit]
Description=scratch web UI
After=default.target

[Service]
ExecStart=%s --port %s
Restart=on-failure
RestartSec=2s

[Install]
WantedBy=default.target
`, bin(), port())
	if err := os.WriteFile(unitPath(), []byte(unit), 0o644); err != nil {
		return err
	}
	if err := sh.RunV("systemctl", "--user", "daemon-reload"); err != nil {
		return err
	}
	return sh.RunV("systemctl", "--user", "enable", service)
}

func Deploy() error {
	mg.SerialDeps(Build, Service, Restart, Status)
	return nil
}

func Restart() error {
	return sh.RunV("systemctl", "--user", "restart", service)
}

func Status() error {
	return sh.RunV("systemctl", "--user", "--no-pager", "status", service)
}

func Logs() error {
	return sh.RunV("journalctl", "--user", "-u", service, "-f")
}

func Uninstall() error {
	sh.RunV("systemctl", "--user", "disable", "--now", service)
	if err := os.Remove(unitPath()); err != nil && !os.IsNotExist(err) {
		return err
	}
	return sh.RunV("systemctl", "--user", "daemon-reload")
}
