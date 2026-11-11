package a

import "testing"

func TestTwo(t *testing.T) {
	t.Run("works", func(t *testing.T) {})
}

func TestExtra(t *testing.T) { // want `one Test per file: two_test.go already declares TestTwo` `TestExtra must be named after its file two_test.go, like TestTwo`
	t.Run("works", func(t *testing.T) {})
}
