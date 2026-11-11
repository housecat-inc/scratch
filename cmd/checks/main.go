package main

import (
	"fmt"
	"os"

	errorscheck "github.com/housecat-inc/scratch/cmd/checks/errors"
	"github.com/housecat-inc/scratch/cmd/checks/testspec"
	"golang.org/x/tools/go/analysis/multichecker"
)

func main() {
	if len(os.Args) == 3 && os.Args[1] == "spec" {
		if err := spec(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	multichecker.Main(errorscheck.Analyzer, testspec.Analyzer)
}

func spec(root string) error {
	pkgs, err := testspec.Tree(root)
	if err != nil {
		return err
	}
	return testspec.Write(os.Stdout, pkgs)
}
