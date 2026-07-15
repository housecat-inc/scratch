package uikit

import (
	"strconv"
	"strings"

	"github.com/a-h/templ"
)

type DataTableColumn struct {
	Header string
	Icon   string
	Sticky bool
	Width  string
}

type DataTableRow struct {
	Cells     []templ.Component
	Href      string
	ID        string
	LinkAttrs templ.Attributes
}

type DataTableProps struct {
	BaseHref string
	Columns  []DataTableColumn
	Empty    string
	ID       string
	Page     int
	PerPage  int
	Rows     []DataTableRow
	Total    int
}

func colAt(cols []DataTableColumn, i int) DataTableColumn {
	if i >= 0 && i < len(cols) {
		return cols[i]
	}
	return DataTableColumn{}
}

func dtEmpty(msg string) string {
	if msg == "" {
		return "Nothing here."
	}
	return msg
}

func dtGridAttrs(cols []DataTableColumn) templ.Attributes {
	tracks := make([]string, 0, len(cols))
	for _, c := range cols {
		w := c.Width
		if w == "" {
			w = "minmax(10rem, 1fr)"
		}
		tracks = append(tracks, w)
	}
	return templ.Attributes{"style": "grid-template-columns: " + strings.Join(tracks, " ")}
}

func dtCellClass(col DataTableColumn) string {
	if col.Sticky {
		return "dt-cell dt-cell-sticky"
	}
	return "dt-cell"
}

func dtHeadCellClass(col DataTableColumn) string {
	if col.Sticky {
		return "dt-head-cell dt-cell-sticky"
	}
	return "dt-head-cell"
}

func dtTotalPages(p DataTableProps) int {
	if p.PerPage <= 0 {
		return 1
	}
	pages := (p.Total + p.PerPage - 1) / p.PerPage
	if pages < 1 {
		return 1
	}
	return pages
}

func dtPageHref(base string, page int) string {
	if page < 1 {
		page = 1
	}
	sep := "?"
	if strings.Contains(base, "?") {
		sep = "&"
	}
	return base + sep + "page=" + strconv.Itoa(page)
}

func dtPageWindow(page, total int) []int {
	const span = 5
	start := page - span/2
	if start < 1 {
		start = 1
	}
	end := start + span - 1
	if end > total {
		end = total
		start = end - span + 1
		if start < 1 {
			start = 1
		}
	}
	out := make([]int, 0, end-start+1)
	for i := start; i <= end; i++ {
		out = append(out, i)
	}
	return out
}

func dtRangeLabel(p DataTableProps) string {
	if p.Total == 0 {
		return "0"
	}
	start := (p.Page-1)*p.PerPage + 1
	end := p.Page * p.PerPage
	if end > p.Total {
		end = p.Total
	}
	return strconv.Itoa(start) + "–" + strconv.Itoa(end) + " of " + strconv.Itoa(p.Total)
}
