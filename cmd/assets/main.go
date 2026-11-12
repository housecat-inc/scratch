package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
	"github.com/evanw/esbuild/pkg/api"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := npm("install", "--no-audit", "--no-fund"); err != nil {
		return err
	}
	if err := bundle(); err != nil {
		return err
	}
	if err := templuiSources(); err != nil {
		return err
	}
	if err := copyFile("ui/node_modules/@klinking/squircle/dist/pill-shape.worklet.mjs", "ui/dist/pill-shape.worklet.mjs"); err != nil {
		return err
	}
	return npm("exec", "--", "@tailwindcss/cli", "-i", "app/css/input.css", "-o", "dist/ui.css", "--minify")
}

func bundle() error {
	r := api.Build(api.BuildOptions{
		Bundle:            true,
		EntryPoints:       []string{"ui/app/browser/index.ts"},
		Format:            api.FormatIIFE,
		LogLevel:          api.LogLevelWarning,
		MinifyIdentifiers: true,
		MinifySyntax:      true,
		MinifyWhitespace:  true,
		Outfile:           "ui/dist/ui.js",
		Target:            api.ES2022,
		Write:             true,
	})
	if len(r.Errors) == 0 {
		return nil
	}
	msgs := make([]string, 0, len(r.Errors))
	for _, e := range r.Errors {
		loc := ""
		if e.Location != nil {
			loc = fmt.Sprintf("%s:%d: ", e.Location.File, e.Location.Line)
		}
		msgs = append(msgs, loc+e.Text)
	}
	return errors.Newf("bundle ui.js:\n%s", strings.Join(msgs, "\n"))
}

func templuiSources() error {
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/templui/templui").Output()
	if err != nil {
		return errors.Wrap(err, "locate templui module")
	}
	var css strings.Builder
	for _, component := range []string{"badge", "button", "card"} {
		fmt.Fprintf(&css, "@source %q;\n", filepath.Join(strings.TrimSpace(string(out)), "components", component))
	}
	return errors.Wrap(os.WriteFile("ui/app/css/templui.css", []byte(css.String()), 0o644), "write templui sources")
}

func npm(args ...string) error {
	cmd := exec.Command("npm", args...)
	cmd.Dir = "ui"
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return errors.Wrapf(cmd.Run(), "npm %s", strings.Join(args, " "))
}

func copyFile(from, to string) error {
	b, err := os.ReadFile(from)
	if err != nil {
		return errors.Wrapf(err, "read %s", from)
	}
	return errors.Wrapf(os.WriteFile(to, b, 0o644), "write %s", to)
}
