package main

import (
	"fmt"
	"strings"
)

type Skip struct {
	Column  int
	File    string
	Line    int
	Literal string
	Safety  Safety
	Sorted  string
}

// A class that survives TwMerge on one side but not the other is, by
// definition, one half of a conflicting pair, so the check already knows
// enough to name the culprits rather than just reporting that something moved.
func (s Skip) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s:%d:%d: skipped, sorting would change the result\n", s.File, s.Line, s.Column)
	fmt.Fprintf(&b, "    literal   %q\n", s.Literal)
	fmt.Fprintf(&b, "    sorted    %q\n", s.Sorted)
	switch {
	case len(s.Safety.Lost) > 0 && len(s.Safety.Gained) > 0:
		fmt.Fprintf(&b, "    conflict  %s and %s resolve to the same tailwind-merge group\n",
			join(s.Safety.Lost), join(s.Safety.Gained))
		fmt.Fprintf(&b, "              as written: %s wins        if sorted: %s would win\n",
			join(s.Safety.Lost), join(s.Safety.Gained))
		fmt.Fprintf(&b, "    fix       one of the two is dead; remove it and the literal sorts cleanly\n")
	case len(s.Safety.Gained) > 0:
		fmt.Fprintf(&b, "    conflict  %s is suppressed as written, but would take effect if sorted\n",
			join(s.Safety.Gained))
		fmt.Fprintf(&b, "    fix       %s is dead today; delete it, or move it after whatever\n", join(s.Safety.Gained))
		fmt.Fprintf(&b, "              overrides it if it was meant to apply\n")
	default:
		fmt.Fprintf(&b, "    conflict  %s takes effect as written, but would be suppressed if sorted\n",
			join(s.Safety.Lost))
		fmt.Fprintf(&b, "    fix       whatever overrides %s is dead today; delete one of them\n", join(s.Safety.Lost))
	}
	return b.String()
}

func join(classes []string) string {
	switch len(classes) {
	case 0:
		return "(none)"
	case 1:
		return classes[0]
	default:
		return strings.Join(classes[:len(classes)-1], ", ") + " and " + classes[len(classes)-1]
	}
}
