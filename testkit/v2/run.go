package testkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type Case[In, Out any] struct {
	Name string
	In   In
	Out  Out
	Err  string
}

func Run[In, Out any](t *testing.T, cases []Case[In, Out], fn func(In) (Out, error)) {
	t.Helper()
	for _, c := range cases {
		t.Run(name(c.Name, c.In), func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			out, err := fn(c.In)
			if c.Err != "" {
				r.ErrorContains(err, c.Err)
				return
			}

			r.NoError(err)
			a.Equal(c.Out, out)
		})
	}
}

func name(explicit string, in any) string {
	if explicit != "" {
		return explicit
	}
	return fmt.Sprint(in)
}
