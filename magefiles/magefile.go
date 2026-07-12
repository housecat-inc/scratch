//go:build mage

package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/magefile/mage/mg"
	"github.com/magefile/mage/sh"
)

var Default = Build

const (
	pkg     = "./cmd/scratch"
	service = "scratch.service"
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
	mg.Deps(devApp, devScratch)
}

func devApp() error {
	return sh.RunV("go", "tool", "air", "-c", "cmd/app/.air.toml")
}

func devScratch() error {
	return sh.RunV("go", "tool", "air", "-c", "cmd/scratch/.air.toml")
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
