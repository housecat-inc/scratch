package workflow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkflow(t *testing.T) {
	t.Run("Greet", func(t *testing.T) {
		r := require.New(t)
		w, err := New(filepath.Join(t.TempDir(), "workflows.db"))
		r.NoError(err)
		defer w.Close()
		r.NoError(w.Launch())

		tests := []struct {
			in   string
			name string
			want string
		}{
			{in: "", name: "greets an empty name", want: "Hello, !"},
			{in: "Ada", name: "greets by name", want: "Hello, Ada!"},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				got, err := w.Greet(tt.in)
				r.NoError(err)
				a.Equal(tt.want, got)
			})
		}
	})
}
