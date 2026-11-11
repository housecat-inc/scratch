package avatar

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	edgePunct = regexp.MustCompile(`^[^\p{L}\p{N}]+|[^\p{L}\p{N}]+$`)
	emailSeps = regexp.MustCompile(`[._+\-]+`)
)

// noise is what a name carries that is not a name: honorifics before it and
// qualifications after. Keys have their dots stripped, so "Dr." and "Dr" match
// the same entry.
var noise = set("dr", "esq", "jr", "md", "miss", "mr", "mrs", "ms", "mx", "phd", "prof", "sir", "sr")

// particles join the parts of a surname. One in the middle of a longer name
// is dropped, so "Ludwig van Beethoven" gives LB; one at an edge is kept, since
// it is then the whole of a name part.
var particles = set("af", "al", "bin", "da", "de", "del", "della", "den", "der", "di", "du", "el", "ibn", "la", "le", "of", "van", "von", "y")

func set(words ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(words))
	for _, w := range words {
		out[w] = struct{}{}
	}
	return out
}

// Initials is up to max letters standing for a person: the first letter of
// each name part, with honorifics and suffixes ignored and surname particles
// skipped, or the first two letters of a lone name. An email address is read
// by its local part, split on its separators, so chris.klink@example.com gives
// CK; only an input with no whitespace counts as an address, since a display
// name that failed to parse can carry an '@' in the middle of a sentence. When
// too many parts remain the first max-1 are kept with the last, so a long name
// still ends on its surname.
func Initials(input string, max int) string {
	if max < 1 {
		max = 3
	}
	s := strings.TrimSpace(input)
	address := strings.IndexByte(s, '@') >= 0 && !strings.ContainsFunc(s, unicode.IsSpace)
	if address {
		s = emailSeps.ReplaceAllString(s[:strings.IndexByte(s, '@')], " ")
	}

	var tokens []string
	for _, t := range strings.Fields(s) {
		t = edgePunct.ReplaceAllString(t, "")
		if t == "" {
			continue
		}
		key := strings.ReplaceAll(strings.ToLower(t), ".", "")
		if _, ok := noise[key]; ok {
			continue
		}
		tokens = append(tokens, t)
	}

	// A display name that failed to parse can still carry its address as one
	// of its words; beside a name, that word is not a name part.
	if len(tokens) > 1 {
		kept := tokens[:0]
		for _, t := range tokens {
			if !strings.ContainsRune(t, '@') {
				kept = append(kept, t)
			}
		}
		tokens = kept
	}

	if len(tokens) > 2 {
		kept := tokens[:0]
		for i, t := range tokens {
			if _, ok := particles[strings.ToLower(t)]; ok && i != 0 && i != len(tokens)-1 {
				continue
			}
			kept = append(kept, t)
		}
		tokens = kept
	}

	switch len(tokens) {
	case 0:
		g := graphemes(strings.TrimSpace(input), 1)
		if address || len(g) == 0 || strings.TrimSpace(g[0]) == "" {
			return "?"
		}
		return strings.ToUpper(g[0])
	case 1:
		return strings.Join(alnumGraphemes(tokens[0], 2), "")
	}

	if len(tokens) > max {
		tokens = append(tokens[:max-1:max-1], tokens[len(tokens)-1])
	}

	var b strings.Builder
	for _, t := range tokens {
		b.WriteString(strings.ToUpper(graphemes(t, 1)[0]))
	}
	return b.String()
}

// graphemes is the first n user-perceived characters of s. A base rune takes
// the combining marks, joiners and variation selectors after it with it, so an
// accented letter or a joined emoji is one cluster rather than several.
func graphemes(s string, n int) []string {
	var out []string
	var current strings.Builder
	for _, r := range s {
		if current.Len() > 0 && !extends(r) {
			out = append(out, current.String())
			if len(out) == n {
				return out
			}
			current.Reset()
		}
		current.WriteRune(r)
	}
	if current.Len() > 0 && len(out) < n {
		out = append(out, current.String())
	}
	return out
}

// FirstLetter is the first letter or digit of s, upper-cased, or "" when it
// has none.
func FirstLetter(s string) string {
	g := alnumGraphemes(s, 1)
	if len(g) == 0 {
		return ""
	}
	return strings.ToUpper(g[0])
}

// alnumGraphemes is the first n graphemes of s that start with a letter or a
// digit, so the apostrophe in O'Brien and the dot in A.J. never stand as an
// initial.
func alnumGraphemes(s string, n int) []string {
	var out []string
	for _, g := range graphemes(s, len(s)) {
		r, _ := utf8.DecodeRuneInString(g)
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			continue
		}
		out = append(out, g)
		if len(out) == n {
			break
		}
	}
	return out
}

func extends(r rune) bool {
	return unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Mc, r) ||
		r == 0x200D || (r >= 0xFE00 && r <= 0xFE0F) || (r >= 0x1F3FB && r <= 0x1F3FF)
}
