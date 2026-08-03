package testkit_test

import (
	"strconv"
	"strings"
	"testing"

	testkit "github.com/housecat-inc/scratch/testkit/v2"
)

func TestUpperGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Case[string, string]{
		{In: "one", Out: "ONE"},
		{In: "two", Out: "TWO"},
	}, func(s string) (string, error) { return strings.ToUpper(s), nil })
}

func TestAtoiGeneric(t *testing.T) {
	testkit.Run(t, []testkit.Case[string, int]{
		{In: "1", Out: 1},
		{In: "a", Err: "invalid syntax"},
	}, strconv.Atoi)
}
