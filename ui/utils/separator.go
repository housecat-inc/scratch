package utils

// SeparatorAfter returns sep for every item in a list but the last, for a
// caller rendering a text separator between elements with no whitespace
// before it.
//
// It exists because of how templ treats whitespace: any whitespace between two
// nodes, a newline included, is rendered as one space, so a separator written
// on its own line always sits one space off the element before it. The only
// place text stays flush is on the same line as that element's closing tag,
// and nothing conditional can share a line with a tag -- but a string
// expression can, and templ fmt leaves it there:
//
//	for i, item := range items {
//		<span>
//			@Item(item)
//		</span>{ utils.SeparatorAfter(i, len(items), ", ") }
//	}
//
// The wrapper element is what makes this work: templ fmt moves text off a
// component call's line, so the expression has to follow an element, not a
// call.
func SeparatorAfter(i, n int, sep string) string {
	if i < n-1 {
		return sep
	}
	return ""
}
