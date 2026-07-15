package todo

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const (
	contactSearchLimit = 20
	contactsPerPage    = 25
)

func (s *WebServer) handleContacts(w http.ResponseWriter, r *http.Request) {
	if s.contacts == nil {
		http.NotFound(w, r)
		return
	}
	total, err := s.contacts.CountContacts()
	if err != nil {
		s.fail(w, err)
		return
	}
	page := contactsPageParam(r.URL.Query().Get("page"), total)
	items, err := s.contacts.ListContactsPage(contactsPerPage, (page-1)*contactsPerPage)
	if err != nil {
		s.fail(w, err)
		return
	}
	props := ui.ContactsProps{
		Contacts: items,
		Counts:   s.navCounts(total),
		Page:     page,
		PerPage:  contactsPerPage,
		Total:    total,
	}
	if r.Header.Get("HX-Request") != "" {
		s.render(w, r, ui.ContactsTable(props))
		return
	}
	s.render(w, r, ui.ContactsPage(props))
}

func (s *WebServer) handleContactSearch(w http.ResponseWriter, r *http.Request) {
	if s.contacts == nil {
		http.NotFound(w, r)
		return
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := s.contactMatches(query)
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, ui.ContactSearchResults(items, query))
}

func (s *WebServer) handleContactCreate(w http.ResponseWriter, r *http.Request) {
	if s.contacts == nil {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseForm(); err != nil {
		s.fail(w, err)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	contact, err := s.contacts.AddContact(name, "", "")
	if err != nil {
		s.fail(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":   strconv.FormatInt(contact.ID, 10),
		"name": contact.Name,
	})
}

func (s *WebServer) contactMatches(query string) ([]db.ContactListItem, error) {
	if query == "" {
		return s.contacts.ListContactsPage(contactSearchLimit, 0)
	}
	return s.contacts.SearchContacts(query, contactSearchLimit)
}

func (s *WebServer) navCounts(contactCount int) ui.NavCounts {
	counts := ui.NavCounts{Contacts: contactCount}
	all, err := s.tasks.All()
	if err != nil {
		return counts
	}
	for _, task := range all {
		if !task.Archived && !task.Completed {
			counts.Inbox++
		}
	}
	counts.Tasks = len(all)
	return counts
}

func contactsPageParam(raw string, total int) int {
	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return 1
	}
	pages := (total + contactsPerPage - 1) / contactsPerPage
	if pages < 1 {
		pages = 1
	}
	if page > pages {
		return pages
	}
	return page
}
