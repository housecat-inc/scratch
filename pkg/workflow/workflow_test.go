package workflow

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGreet(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty", in: "", want: "Hello, !"},
		{name: "name", in: "Ada", want: "Hello, Ada!"},
	}

	r := require.New(t)
	w, err := New(filepath.Join(t.TempDir(), "workflows.db"))
	r.NoError(err)
	defer w.Close()
	r.NoError(w.Launch())

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := w.Greet(tt.in)
			a.NoError(err)
			a.Equal(tt.want, got)
		})
	}
}
