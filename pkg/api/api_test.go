package api

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpec(t *testing.T) {
	r := require.New(t)
	spec := Spec(filepath.Join(t.TempDir(), "openapi.json"))
	r.NotNil(spec)

	tests := []struct {
		name string
		path string
	}{
		{name: "greet", path: "/api/workflows/greet"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			item := spec.Paths.Find(tt.path)
			a.NotNil(item)
			a.NotNil(item.Post)
			a.Equal("Run the durable greet workflow", item.Post.Summary)
		})
	}
}
