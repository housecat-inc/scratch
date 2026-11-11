package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

// git exports GIT_DIR and GIT_INDEX_FILE when it runs a hook, and with those
// set rev-parse resolves against the current directory rather than discovering
// the repository, so --show-toplevel comes back as whatever directory it was
// handed. Dropping them makes the answer the same from a hook as from a shell.
func repoRoot(dir string) (string, error) {
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	cmd.Env = []string{}
	for _, entry := range os.Environ() {
		switch {
		case strings.HasPrefix(entry, "GIT_DIR="),
			strings.HasPrefix(entry, "GIT_WORK_TREE="),
			strings.HasPrefix(entry, "GIT_INDEX_FILE="):
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}

	out, err := cmd.Output()
	if err != nil {
		return "", errors.Wrap(err, "git rev-parse --show-toplevel")
	}
	return strings.TrimSpace(string(out)), nil
}

// auth is a separate module with its own stylesheet. app's entrypoint already
// @sources the ui module, so everything outside auth resolves to it.
func cssEntryFor(root, file string) string {
	relative, err := filepath.Rel(root, file)
	if err == nil && strings.HasPrefix(filepath.ToSlash(relative), "auth/") {
		return filepath.Join(root, "auth/assets/css/input.css")
	}
	return filepath.Join(root, "app/assets/css/input.css")
}

func main() {
	strict := flag.Bool("strict", false, "exit non-zero when a literal is skipped")
	flag.Parse()

	if err := run(flag.Args(), *strict); err != nil {
		fmt.Fprintln(os.Stderr, "twsort:", err)
		os.Exit(1)
	}
}

func run(files []string, strict bool) error {
	if len(files) == 0 {
		return nil
	}

	root, err := repoRoot(filepath.Dir(files[0]))
	if err != nil {
		return errors.Wrap(err, "locate repo root")
	}

	skipped, filesWithSkips := 0, 0
	for _, file := range files {
		source, err := os.ReadFile(file)
		if err != nil {
			return errors.Wrapf(err, "read %s", file)
		}

		out, skips, err := Rewrite(RewriteIn{
			CSSEntry: cssEntryFor(root, file),
			File:     file,
			Source:   string(source),
		})
		if err != nil {
			return errors.Wrapf(err, "rewrite %s", file)
		}

		for _, skip := range skips {
			fmt.Fprint(os.Stderr, skip.String())
		}
		if len(skips) > 0 {
			skipped += len(skips)
			filesWithSkips++
		}

		if out != string(source) {
			if err := os.WriteFile(file, []byte(out), 0o644); err != nil {
				return errors.Wrapf(err, "write %s", file)
			}
		}
	}

	// A skip does not fail the run: a pre-existing conflict in a file must not
	// block every future commit that touches it.
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "twsort: %s skipped in %s\n", plural(skipped, "literal"), plural(filesWithSkips, "file"))
		if strict {
			return errors.New("unsafe literals skipped")
		}
	}
	return nil
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
