package main

import (
	"fmt"
	"os"

	"github.com/cockroachdb/errors"

	errorscheck "github.com/housecat-inc/scratch/cmd/checks/errors"
	"github.com/housecat-inc/scratch/cmd/checks/testspec"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "lint" {
		if err := lint(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "spec" {
		if err := spec(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	multichecker.Main(errorscheck.Analyzer, testspec.Analyzer)
}

func lint(dir string) error {
	problems, err := testspec.Lint(dir)
	if err != nil {
		return err
	}
	for _, p := range problems {
		fmt.Println(p)
	}
	if len(problems) > 0 {
		return errors.Errorf("%d testspec problems", len(problems))
	}
	return nil
}

func spec(root string) error {
	pkgs, err := testspec.Tree(root)
	if err != nil {
		return err
	}
	return testspec.Write(os.Stdout, pkgs)
}
