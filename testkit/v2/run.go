package testkit

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type T struct {
	*testing.T
	A *assert.Assertions
	R *require.Assertions
}

type Test[In, Out any] struct {
	Name string
	In   In
	Out  Out
	Err  string
}

type Option func(*options)

type options struct {
	setup func(t *T)
}

func Setup(fn func(t *T)) Option { return func(o *options) { o.setup = fn } }

func Run[In, Out any](t *testing.T, tests []Test[In, Out], fn func(In) (Out, error), opts ...Option) {
	t.Helper()
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	for _, tt := range tests {
		t.Run(name(tt.Name, tt.In), func(t *testing.T) {
			tk := &T{T: t, A: assert.New(t), R: require.New(t)}

			if o.setup != nil {
				o.setup(tk)
			}

			out, err := fn(tt.In)
			if tt.Err != "" {
				tk.R.ErrorContains(err, tt.Err)
				return
			}

			tk.R.NoError(err)
			tk.A.Equal(tt.Out, out)
		})
	}
}

func name(name string, in any) string {
	if name != "" {
		return name
	}
	return fmt.Sprint(in)
}
