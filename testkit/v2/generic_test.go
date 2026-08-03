package testkit_test

import (
	"bytes"
	"strconv"
	"strings"
	"testing"

	testkit "github.com/housecat-inc/scratch/testkit/v2"
)

func TestUpperGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Test[string, string]{
		{In: "one", Out: "ONE"},
		{In: "two", Out: "TWO"},
	}, func(s string) (string, error) { return strings.ToUpper(s), nil })
}

func TestAtoiGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Test[string, int]{
		{In: "1", Out: 1},
		{In: "a", Err: "invalid syntax"},
	}, strconv.Atoi)
}

func TestSetupTeardownGeneric(t *testing.T) {
	var buf *bytes.Buffer

	testkit.Run(t, []testkit.Test[string, int]{
		{In: "world", Out: 11, Setup: func(t *testkit.T) { buf.WriteString("hello ") }},
		{In: "there", Out: 5},
	}, func(s string) (int, error) {
		buf.WriteString(s)
		return buf.Len(), nil
	},
		testkit.Setup(func(t *testkit.T) { buf = &bytes.Buffer{} }),
		testkit.Teardown(func(t *testkit.T) { t.A.NotZero(buf.Len()) }),
	)
}

func TestSplitGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Test[string, []string]{
		{
			In: "a,b,c",
			Check: func(t *testkit.T, out []string) {
				t.R.Len(out, 3)
				t.A.Equal("a", out[0])
				t.A.Equal("c", out[2])
			},
		},
	}, func(s string) ([]string, error) { return strings.Split(s, ","), nil })
}
