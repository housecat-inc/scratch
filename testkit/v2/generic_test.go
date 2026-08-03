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
		{In: "hello", Out: 5},
		{In: "hi", Out: 2},
	}, func(s string) (int, error) { return buf.WriteString(s) },
		testkit.Setup(func(t *testkit.T) {
			buf = &bytes.Buffer{}
			t.Cleanup(func() { t.A.NotZero(buf.Len()) })
		}),
	)
}

func TestSplitGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Test[string, []string]{
		{In: "a,b,c", Out: []string{"a", "b", "c"}},
		{In: "x", Out: []string{"x"}},
	}, func(s string) ([]string, error) { return strings.Split(s, ","), nil })
}
