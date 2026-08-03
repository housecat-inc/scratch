package testkit

import (
	"fmt"
	"testing"

	tassert "github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type T struct {
	*testing.T
	A *tassert.Assertions
	R *require.Assertions
}

type Test[In, Out any] struct {
	Name  string
	In    In
	Out   Out
	Err   string
	Check func(t *T, out Out)
}

func (tt Test[In, Out]) name() string {
	if tt.Name != "" {
		return tt.Name
	}
	return fmt.Sprint(tt.In)
}

func Run[In, Out any](t *testing.T, tests []Test[In, Out], fn func(In) (Out, error), opts ...Option) {
	t.Helper()
	RunF(t,
		func(*T) struct{} { return struct{}{} },
		tests,
		func(_ *T, _ struct{}, in In) (Out, error) { return fn(in) },
		opts...,
	)
}

func RunF[Fix, In, Out any](t *testing.T, setup func(t *T) Fix, tests []Test[In, Out], fn func(t *T, fix Fix, in In) (Out, error), opts ...Option) {
	t.Helper()
	o := newOptions(opts)
	for _, tt := range tests {
		t.Run(tt.name(), func(t *testing.T) {
			if o.parallel {
				t.Parallel()
			}
			tk := &T{T: t, A: tassert.New(t), R: require.New(t)}
			fix := setup(tk)
			out, err := fn(tk, fix, tt.In)
			assert(tk, tt, out, err)
		})
	}
}

func Pure[In, Out any](fn func(In) Out) func(In) (Out, error) {
	return func(in In) (Out, error) { return fn(in), nil }
}

func assert[In, Out any](tk *T, tt Test[In, Out], out Out, err error) {
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
