package a

import "testing"

func TestDeep(t *testing.T) {
	t.Run("Group", func(t *testing.T) {
		t.Run("case", func(t *testing.T) {
			t.Run("too deep", func(t *testing.T) {}) // want `tests nest at most three levels: Test, group, case`
		})
	})
}
