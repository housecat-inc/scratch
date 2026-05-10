package agents

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDir(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	t.Setenv("HOME", "/tmp/home")
	dir, err := Dir()
	r.NoError(err)
	a.Equal(filepath.Join("/tmp/home", "scratch"), dir)
}
