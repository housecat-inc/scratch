package ui

import (
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
)

type ContactsProps struct {
	Contacts []db.ContactListItem
	Counts   NavCounts
	Page     int
	PerPage  int
	Total    int
}

func contactInitials(name string) string {
	fields := strings.Fields(name)
	if len(fields) == 0 {
		return "?"
	}
	if len(fields) == 1 {
		return strings.ToUpper(fields[0][:1])
	}
	return strings.ToUpper(fields[0][:1] + fields[len(fields)-1][:1])
}

func contactDash(s string) string {
	if strings.TrimSpace(s) == "" {
		return "—"
	}
	return s
}

func contactCreateLabel(query string) string {
	return `+ Create "` + query + `"`
}

func contactStatusSlug(status string) string {
	return strings.ToLower(strings.ReplaceAll(status, " ", "-"))
}
