package a

import "testing"

func RunAll(t *testing.T, names []string) {
	for _, name := range names {
		t.Run(name, func(t *testing.T) {}) // want `t.Run must be called inside a _test.go file's Test function, not a helper`
	}
}

func TestLooksLikeATest(t *testing.T) {
	t.Run("in a non-test file", func(t *testing.T) {}) // want `t.Run must be called inside a _test.go file's Test function, not a helper`
}
