package testkit

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type T struct {
	A         *assert.Assertions
	Artifacts *Artifacts
	R         *require.Assertions
	t         *testing.T
}

func New(t *testing.T) *T {
	t.Helper()

	return &T{
		A:         assert.New(t),
		Artifacts: NewArtifacts(t),
		R:         require.New(t),
		t:         t,
	}
}

func (t *T) JSON(got any) []string {
	t.t.Helper()
	return JSON(got)
}

func (t *T) Equal(expected any, actual any, msgAndArgs ...any) bool {
	t.t.Helper()
	return t.A.Equal(expected, actual, msgAndArgs...)
}

func (t *T) NoError(err error, msgAndArgs ...any) {
	t.t.Helper()
	t.R.NoError(err, msgAndArgs...)
}
