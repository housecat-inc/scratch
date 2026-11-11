// Package collection supplies headless ARIA grid parts for interactive lists
// and tables. Callers keep their own elements and presentation.
package collection

import "github.com/a-h/templ"

type Props struct {
	// EnterFromPage lets Up and Down from anywhere off a list enter it, for a
	// page's main list of entries, such as Mail's. Other lists are entered by
	// Tab, so the arrows still scroll the page around them.
	EnterFromPage bool
	HoverCurrent  bool
	// RowNavigation moves through rows, with their controls as stops
	// (data-navigation="row"); off, the grid is cell-oriented
	// (data-navigation="cell"). The README describes both modes.
	RowNavigation bool
}

// navigation is the mode a root renders, always present: a cell-oriented
// grid is a mode of its own, not the absence of row navigation.
func navigation(p Props) string {
	if p.RowNavigation {
		return "row"
	}
	return "cell"
}

func Root(label string, props ...Props) templ.Attributes {
	var p Props
	if len(props) > 0 {
		p = props[0]
	}
	return templ.Attributes{"aria-label": label, "data-collection": "root", "data-enter-from-page": p.EnterFromPage, "data-hover-current": p.HoverCurrent, "data-navigation": navigation(p), "role": "grid"}
}

func RowGroup(props ...Props) templ.Attributes {
	attrs := Root("", props...)
	delete(attrs, "aria-label")
	attrs["role"] = "rowgroup"
	return attrs
}

func Row() templ.Attributes {
	return templ.Attributes{"data-collection": "row", "role": "row"}
}

func Cell() templ.Attributes {
	return templ.Attributes{"data-collection": "cell", "role": "gridcell", "tabindex": "-1"}
}

func Header() templ.Attributes {
	return templ.Attributes{"data-collection": "cell", "role": "columnheader", "tabindex": "-1"}
}

// RowHeader is a header cell that labels its own row, such as a section row
// in the Mail settings sidebar. Unlike a column header it joins the navigation.
func RowHeader() templ.Attributes {
	return templ.Attributes{"data-collection": "cell", "role": "rowheader", "tabindex": "-1"}
}

func Trigger() templ.Attributes {
	return templ.Attributes{"data-collection": "trigger"}
}

func PrimaryAction() templ.Attributes {
	return templ.Attributes{"data-collection": "action"}
}
