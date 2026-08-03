package testkit_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Example of idiomatic table driven test https://go.dev/wiki/TableDrivenTests
func TestUpper(t *testing.T) {
	var tests = []struct {
		in  string
		out string
	}{
		{"one", "ONE"},
		{"two", "TWO"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			a := assert.New(t)
			a.Equal(tt.out, strings.ToUpper(tt.in))
		})
	}
}

func TestAtoi(t *testing.T) {
	var tests = []struct {
		in  string
		out int
		err string
	}{
		{"1", 1, ""},
		{"a", 0, "invalid syntax"},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			out, err := strconv.Atoi(tt.in)
			if tt.err != "" {
				r.ErrorContains(err, tt.err)
				return
			}
			r.NoError(err)

			a.Equal(tt.out, out)
		})
	}
}

func TestSetupTeardown(t *testing.T) {
	var tests = []struct {
		in  string
		out int
	}{
		{"hello", 5},
		{"hi", 2},
	}

	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			buf := &bytes.Buffer{}
			t.Cleanup(func() { a.NotZero(buf.Len()) })

			out, err := buf.WriteString(tt.in)
			r.NoError(err)

			a.Equal(tt.out, out)
		})
	}
}
