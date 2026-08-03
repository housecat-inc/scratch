package testkit

import (
	"fmt"
	"strings"
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
	Name   string
	In     In
	Out    Out
	Err    string
	Assert func(t *T, out Out, err error)
}

var nameReplacer = strings.NewReplacer(" ", "_", "/", "_", "\t", "_", "\n", "_")

func (tt Test[In, Out]) name() string {
	if tt.Name != "" {
		return tt.Name
	}
	return nameReplacer.Replace(fmt.Sprint(tt.In))
}

func Run[In, Out any](t *testing.T, tests []Test[In, Out], fn func(In) (Out, error), opts ...Option) {
	t.Helper()
	RunF(t,
		tests,
		func(*T) struct{} { return struct{}{} },
		func(_ *T, _ struct{}, in In) (Out, error) { return fn(in) },
		opts...,
	)
}

func RunF[Fix, In, Out any](t *testing.T, tests []Test[In, Out], setup func(t *T) Fix, fn func(t *T, fix Fix, in In) (Out, error), opts ...Option) {
	t.Helper()
	o := newOptions(opts)
	for _, tt := range tests {
		t.Run(tt.name(), func(t *testing.T) {
			if o.parallel {
				t.Parallel()
			}
			tk := &T{T: t, A: assert.New(t), R: require.New(t)}
			fix := setup(tk)
			out, err := fn(tk, fix, tt.In)
			check(tk, tt, out, err)
		})
	}
}

func Pure[In, Out any](fn func(In) Out) func(In) (Out, error) {
	return func(in In) (Out, error) { return fn(in), nil }
}

func check[In, Out any](tk *T, tt Test[In, Out], out Out, err error) {
	if tt.Assert != nil {
		tt.Assert(tk, out, err)
		return
	}
	if tt.Err != "" {
		tk.R.ErrorContains(err, tt.Err)
		return
	}
	tk.R.NoError(err)
	tk.A.Equal(tt.Out, out)
}
