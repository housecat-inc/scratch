package main

import "sort"

// Unranked classes go first in their original relative order, matching
// prettier-plugin-tailwindcss. Equal ranks keep source order so the sort is
// stable and the tool is idempotent.
func Sort(classes []string, ranks map[string]int) []string {
	indexed := make([]int, len(classes))
	for i := range classes {
		indexed[i] = i
	}

	sort.SliceStable(indexed, func(x, y int) bool {
		a, b := indexed[x], indexed[y]
		rankA, okA := ranks[classes[a]]
		rankB, okB := ranks[classes[b]]
		switch {
		case !okA && !okB:
			return a < b
		case !okA:
			return true
		case !okB:
			return false
		case rankA != rankB:
			return rankA < rankB
		default:
			return a < b
		}
	})

	out := make([]string, 0, len(classes))
	for _, i := range indexed {
		out = append(out, classes[i])
	}
	return out
}
