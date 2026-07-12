package uikit

import "strings"

type Tab struct {
	Active bool
	Count  int
	Href   string
	Label  string
}

type TabsProps struct {
	Class string
	Tabs  []Tab
}

func tabsClass(extra string) string {
	classes := []string{"gm-tabs"}
	if extra != "" {
		classes = append(classes, extra)
	}
	return strings.Join(classes, " ")
}
