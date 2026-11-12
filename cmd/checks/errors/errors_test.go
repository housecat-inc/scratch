package errorscheck

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	t.Run("reports std and pkg errors but allows cockroachdb errors", func(t *testing.T) {
		analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
	})
}
