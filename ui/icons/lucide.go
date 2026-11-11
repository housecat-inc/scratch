package icons

import (
	"slices"
	"strings"

	"github.com/a-h/templ"

	"github.com/housecat-inc/scratch/ui/icons/internal/lucide"
)

// The semantic Name set pins a glyph to each meaning; the functions here are
// the one deliberate door to an arbitrary Lucide icon, for the picker where a
// user chooses a glyph for their own view. Nothing else should name a Lucide
// icon directly.

const (
	LucidePageSize = 60

	lucidePrefix   = "lucide:"
	lucideQueryMax = 40
)

// The match tiers, best first. A search lists each tier in turn.
const (
	tierNameExact = iota
	tierNamePrefix
	tierNameWord
	tierTagExact
	tierTagWord
	tierNameSubstring
	tierCount
)

func HasLucide(name string) bool {
	return lucide.Has(name)
}

// RenderLucide draws nothing for a name that is not in the set, like Render.
func RenderLucide(name string, props ...Props) templ.Component {
	if !lucide.Has(name) {
		return templ.NopComponent
	}
	return lucide.Icon(name)(props...)
}

// LucideRef and ParseLucideRef are the only place that knows how a chosen
// icon is stored, so storage, validation and rendering cannot drift.
func LucideRef(name string) string {
	return lucidePrefix + name
}

func ParseLucideRef(ref string) (string, bool) {
	name, ok := strings.CutPrefix(ref, lucidePrefix)
	if !ok || !lucide.Has(name) {
		return "", false
	}
	return name, true
}

// SuggestedLucide is what the picker shows before anything is typed. The first
// three are the glyphs of the Important, FYI and Other rows, which users are
// likely to replace with a view of their own. Eight to a row; the picker puts
// the default icon first, so these 63 fill eight rows.
func SuggestedLucide() []string {
	return []string{
		glyphs[Inbox], glyphs[Info], glyphs[Later], "briefcase", "users", "handshake", "receipt",
		"bot", "trash-2", "credit-card", "shopping-cart", "package", "calendar", "bell", "flag",
		"zap", "heart", "plane", "newspaper", "lightbulb", "rocket", "git-branch", "folder",
		"folder-open", "folder-check", "folder-clock", "file-text", "tag", "search", "mail", "message-square",
		"phone", "video", "globe", "house", "building-2", "landmark", "wallet", "banknote",
		"chart-line", "chart-pie", "target", "trophy", "gift", "graduation-cap", "book-open", "pen-line",
		"paperclip", "link", "lock", "shield", "key", "settings", "wrench", "truck",
		"map-pin", "coffee", "music", "camera", "sun", "moon", "cloud", "flame",
	}
}

type LucideSearchIn struct {
	Limit  int
	Offset int
	Query  string
}

type LucideSearchOut struct {
	More  bool
	Names []string
}

// SearchLucide ranks name matches (exact, prefix, word-boundary), then tag
// matches (exact, word-start), then name substring matches, each tier in name
// order, and pages the result. An empty query returns the suggested set with
// no paging.
func SearchLucide(in LucideSearchIn) LucideSearchOut {
	q := normalizeLucideQuery(in.Query)
	if q == "" {
		return LucideSearchOut{Names: SuggestedLucide()}
	}
	limit := in.Limit
	if limit <= 0 {
		limit = LucidePageSize
	}
	offset := max(in.Offset, 0)
	var tiers [tierCount][]string
	for _, name := range lucide.Names() {
		if tier := lucideTier(name, q); tier >= 0 {
			tiers[tier] = append(tiers[tier], name)
		}
	}
	ranked := slices.Concat(tiers[:]...)
	if offset >= len(ranked) {
		return LucideSearchOut{Names: []string{}}
	}
	end := min(offset+limit, len(ranked))
	return LucideSearchOut{More: end < len(ranked), Names: ranked[offset:end]}
}

func normalizeLucideQuery(q string) string {
	runes := []rune(strings.Join(strings.Fields(strings.ToLower(q)), "-"))
	if len(runes) > lucideQueryMax {
		runes = runes[:lucideQueryMax]
	}
	return string(runes)
}

// lucideTier ranks how well q matches an icon, lowest best, or -1 for none.
// A substring of a name outranks nothing but a miss, since "bell" inside
// "dumbbell" is noise next to an icon Lucide itself tags "bell"; tags are
// matched whole or from a word start for the same reason.
func lucideTier(name, q string) int {
	word := "-" + q
	switch {
	case name == q:
		return tierNameExact
	case strings.HasPrefix(name, q):
		return tierNamePrefix
	case strings.Contains(name, word):
		return tierNameWord
	}
	tags := lucide.SearchTags(name)
	if slices.Contains(tags, q) {
		return tierTagExact
	}
	if slices.ContainsFunc(tags, func(tag string) bool { return strings.HasPrefix(tag, q) || strings.Contains(tag, word) }) {
		return tierTagWord
	}
	if strings.Contains(name, q) {
		return tierNameSubstring
	}
	return -1
}
