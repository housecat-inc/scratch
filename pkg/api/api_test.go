package api

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSpec(t *testing.T) {
	spec := Spec(filepath.Join(t.TempDir(), "openapi.json"))
	require.New(t).NotNil(spec)

	tests := []struct {
		name    string
		path    string
		summary string
	}{
		{name: "documents the greet workflow route", path: "/api/workflows/greet", summary: "Run the durable greet workflow"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			item := spec.Paths.Find(tt.path)
			r.NotNil(item)
			r.NotNil(item.Post)
			a.Equal(tt.summary, item.Post.Summary)
		})
	}
}
