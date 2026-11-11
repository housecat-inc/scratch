package a

import "testing"

func TestHelper(t *testing.T) {
	t.Run("works", func(t *testing.T) { each(t) })
	t.Run("named body", body) // want `t.Run body must be a func\(t \*testing.T\) literal`
}

func body(t *testing.T) {}

func each(t *testing.T) {
	t.Run("hidden", func(t *testing.T) {}) // want `t.Run must be called inside a _test.go file's Test function, not a helper`
}
