package testspec

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestTestspec(t *testing.T) {
	t.Run("Analyzer", func(t *testing.T) {
		t.Run("reports every naming and structure violation", func(t *testing.T) {
			analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
		})
	})

	t.Run("Write", func(t *testing.T) {
		t.Run("prints the sorted Test, group, case hierarchy", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			pkgs, err := Tree("testdata/src")
			r.NoError(err)

			var b strings.Builder
			r.NoError(Write(&b, pkgs))
			a.Equal(strings.TrimLeft(`
a
  Banned
    uses runners
  Deep
    Group
      case
        too deep
  Extra
    works
  Flat
  Good
    Add
      accepts one
      accepts zero
    rejects nothing
    when the input is a map
      URL keys stay upper
      counts keys
  Helper
    works
  Names
    Returns an error
    Table
    snake_case
    works
    works
  Two
    works
`, "\n"), b.String())
		})
	})
}
