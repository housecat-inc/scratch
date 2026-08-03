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
	Name  string
	In    In
	Out   Out
	Err   string
	Check func(t *T, out Out)
}

func Run[In, Out any](t *testing.T, tests []Test[In, Out], fn func(In) (Out, error), opts ...Option) {
	t.Helper()
	o := newOptions(opts)
	for _, tt := range tests {
		t.Run(name(tt.Name, tt.In), func(t *testing.T) {
			if o.parallel {
				t.Parallel()
			}
			tk := &T{T: t, A: assert.New(t), R: require.New(t)}
			out, err := fn(tt.In)
			verify(tk, tt, out, err)
		})
	}
}

func RunF[Fix, In, Out any](t *testing.T, setup func(t *T) Fix, tests []Test[In, Out], fn func(t *T, fix Fix, in In) (Out, error), opts ...Option) {
	t.Helper()
	o := newOptions(opts)
	for _, tt := range tests {
		t.Run(name(tt.Name, tt.In), func(t *testing.T) {
			if o.parallel {
				t.Parallel()
			}
			tk := &T{T: t, A: assert.New(t), R: require.New(t)}
			fix := setup(tk)
			out, err := fn(tk, fix, tt.In)
			verify(tk, tt, out, err)
		})
	}
}

func Pure[In, Out any](fn func(In) Out) func(In) (Out, error) {
	return func(in In) (Out, error) { return fn(in), nil }
}

func verify[In, Out any](tk *T, tt Test[In, Out], out Out, err error) {
	if tt.Err != "" {
		tk.R.ErrorContains(err, tt.Err)
		return
	}

	tk.R.NoError(err)
	check := tt.Check
	if check == nil {
		check = func(t *T, out Out) { t.A.Equal(tt.Out, out) }
	}
	check(tk, out)
}

func name(name string, in any) string {
	if name != "" {
		return name
	}
	return fmt.Sprint(in)
}
