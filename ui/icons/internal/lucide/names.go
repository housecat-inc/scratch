package lucide

import (
	"maps"
	"slices"
	"strings"
	"sync"
)

func Has(name string) bool {
	_, ok := internalSvgData[name]
	return ok
}

var Names = sync.OnceValue(func() []string {
	return slices.Sorted(maps.Keys(internalSvgData))
})

var tags = sync.OnceValue(func() map[string][]string {
	out := make(map[string][]string, len(internalTags))
	for name, joined := range internalTags {
		out[name] = strings.Fields(joined)
	}
	return out
})

// SearchTags are the lowercase search words Lucide gives an icon, multi-word tags
// joined by hyphens. Callers must not modify the slice.
func SearchTags(name string) []string {
	return tags()[name]
}
