package ui

import (
	"strconv"
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/uikit"
)

func contactNotesCount(notes []ContactNoteView) string {
	if len(notes) == 0 {
		return ""
	}
	return "(" + strconv.Itoa(len(notes)) + ")"
}

type ContactNoteView struct {
	Body string
	When string
}

type ContactDetailProps struct {
	ChatLabel   string
	ChatOptions []uikit.SelectOption
	Contact     db.Contact
	Counts      NavCounts
	Emails      []db.Email
	Notes       []ContactNoteView
}

type ContactsProps struct {
	ChatLabel   string
	ChatOptions []uikit.SelectOption
	Contacts    []db.ContactListItem
	Counts      NavCounts
	Page        int
	PerPage     int
	Total       int
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
