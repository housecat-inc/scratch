package testkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Test[In, Out any] struct {
	Name string
	In   In
	Out  Out
	Err  string
}

func Run[In, Out any](t *testing.T, tests []Test[In, Out], fn func(In) (Out, error)) {
	t.Helper()
	for _, tt := range tests {
		t.Run(name(tt.Name, tt.In), func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			out, err := fn(tt.In)
			if tt.Err != "" {
				r.ErrorContains(err, tt.Err)
				return
			}

			r.NoError(err)
			a.Equal(tt.Out, out)
		})
	}
}

func name(name string, in any) string {
	if name != "" {
		return name
	}
	return fmt.Sprint(in)
}
