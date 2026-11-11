package testspec

import (
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Run("reports every naming and structure violation", func(t *testing.T) {
		analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	})
}

func TestLint(t *testing.T) {
	t.Run("reports test file problems with positions", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		problems, err := Lint("testdata/src/a")
		r.NoError(err)
		a.Contains(problems, "testdata/src/a/flat_test.go:5:6: TestFlat must describe behavior with t.Run cases")
		a.True(slices.IsSorted(problems))
	})

	t.Run("passes a conforming package", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		problems, err := Lint(".")
		r.NoError(err)
		a.Empty(problems)
	})
}

func TestWrite(t *testing.T) {
	t.Run("prints each file with its sorted Test, group, case hierarchy", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		files, err := Tree("testdata/src")
		r.NoError(err)

		var b strings.Builder
		r.NoError(Write(&b, files))
		a.Equal(strings.TrimLeft(`
a/deep_test.go
  Deep
    when nested
      case
        too deep

a/flat_test.go
  Flat

a/good_test.go
  Good
    rejects nothing
    when the input is a map
      URL keys stay upper
      counts keys
    when the input is a slice
      accepts one
      accepts zero

a/helper_test.go
  Helper
    works

a/names_test.go
  Names
    Returns an error
    snake_case
    when using a table
    works
    works

a/subject_test.go
  Subject
    Close
    MergeDefaults
      keeps unknown fields
    foldParts
      joins text
    inbox
      lists active tasks

a/two_test.go
  Extra
    works
  Two
    works
`, "\n"), b.String())
	})
}
