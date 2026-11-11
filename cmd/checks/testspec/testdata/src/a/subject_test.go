package a

import "testing"

func TestSubject(t *testing.T) {
	t.Run("MergeDefaults", func(t *testing.T) { // want `group "MergeDefaults" names a subject; make it its own TestMergeDefaults`
		t.Run("keeps unknown fields", func(t *testing.T) {})
	})
	t.Run("foldParts", func(t *testing.T) { // want `group "foldParts" names a subject; make it its own TestFoldParts`
		t.Run("joins text", func(t *testing.T) {})
	})
	t.Run("inbox", func(t *testing.T) {
		t.Run("lists active tasks", func(t *testing.T) {})
	})
	t.Run("Close", func(t *testing.T) {})
}
