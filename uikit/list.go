package uikit

import (
	"strings"

	"github.com/a-h/templ"
)

type ListRowProps struct {
	Actions     templ.Component
	Archived    bool
	Class       string
	Done        bool
	Href        string
	ID          string
	Kind        string
	Labels      []string
	Primary     templ.Component
	PrimaryMeta templ.Component
	Subject     string
	What        string
	When        string
}

func listRowClass(p ListRowProps) string {
	classes := []string{"gm-row"}
	if p.Archived {
		classes = append(classes, "archived")
	}
	if p.Class != "" {
		classes = append(classes, p.Class)
	}
	if p.Done {
		classes = append(classes, "done")
	}
	if p.PrimaryMeta != nil {
		classes = append(classes, "has-primary-meta")
	}
	return strings.Join(classes, " ")
}
