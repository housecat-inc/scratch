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
	initial := func(field string) string {
		return string([]rune(field)[:1])
	}
	if len(fields) == 1 {
		return strings.ToUpper(initial(fields[0]))
	}
	return strings.ToUpper(initial(fields[0]) + initial(fields[len(fields)-1]))
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

func ContactValueLabel(store db.ContactStore, value string) string {
	if store == nil {
		return ""
	}
	id, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || id <= 0 {
		return ""
	}
	label, err := store.ContactLabel(id)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(label)
}

func contactOptionLabel(c db.ContactListItem) string {
	return contactNameEmailLabel(c.Name, c.PrimaryEmail)
}

func contactSelectedLabel(field ChatFormFieldProps) string {
	value := strings.TrimSpace(field.Value)
	if value == "" {
		return ""
	}
	if label := strings.TrimSpace(field.ValueLabel); label != "" {
		return label
	}
	return "Contact #" + value
}

func contactNameEmailLabel(name, email string) string {
	name = strings.TrimSpace(name)
	email = strings.TrimSpace(email)
	if email == "" {
		return name
	}
	if name == "" {
		return email
	}
	return name + " / " + email
}

func contactStatusSlug(status string) string {
	return strings.ToLower(strings.ReplaceAll(status, " ", "-"))
}
