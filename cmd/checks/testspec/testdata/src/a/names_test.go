package a

import (
	"fmt"
	"testing"
)

func TestNames(t *testing.T) {
	t.Run("Returns an error", func(t *testing.T) {}) // want `test name "Returns an error" must read like a sentence starting lowercase`
	t.Run("snake_case", func(t *testing.T) {})       // want `test name "snake_case" must read as words: no underscores, slashes or extra spaces`
	t.Run("works", func(t *testing.T) {})
	t.Run("works", func(t *testing.T) {})               // want `duplicate test name "works"`
	t.Run(fmt.Sprint("dynamic"), func(t *testing.T) {}) // want `t.Run name must be a string literal or tt.name from a literal table`

	t.Run("when using a table", func(t *testing.T) {
		tests := []struct {
			in   string
			name string
		}{
			{in: "x"}, // want `table case needs a string literal name field`
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {})
		}
		for _, tt := range tests {
			t.Run(tt.in, func(t *testing.T) {}) // want `t.Run name must be a string literal or tt.name from a literal table`
		}
	})
}
