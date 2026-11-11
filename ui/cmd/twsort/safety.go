package main

import (
	"sort"
	"strings"

	"github.com/housecat-inc/scratch/ui/utils"
)

type Safety struct {
	Gained []string
	Lost   []string
	OK     bool
}

// TwMerge's output ORDER is nondeterministic across processes -- it resolves
// conflicts through a Go map, the same quirk ui/storybook/gen.go:70 works
// around -- so only the surviving SET is comparable. The set is also the only
// part that matters: class attribute order does not affect the cascade.
func Check(original, sorted string) Safety {
	before := survivors(original)
	after := survivors(sorted)

	out := Safety{
		Gained: difference(after, before),
		Lost:   difference(before, after),
	}
	out.OK = len(out.Gained) == 0 && len(out.Lost) == 0
	return out
}

func survivors(classes string) map[string]bool {
	out := map[string]bool{}
	for _, class := range strings.Fields(utils.TwMerge(classes)) {
		out[class] = true
	}
	return out
}

func difference(a, b map[string]bool) []string {
	var out []string
	for class := range a {
		if !b[class] {
			out = append(out, class)
		}
	}
	sort.Strings(out)
	return out
}
