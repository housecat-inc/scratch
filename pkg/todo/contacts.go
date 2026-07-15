package todo

import (
	"net/http"
	"strconv"

	"github.com/housecat-inc/scratch/pkg/ui"
)

const contactsPerPage = 25

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
		Counts:   s.navCounts(),
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

func (s *WebServer) navCounts() ui.NavCounts {
	counts := ui.NavCounts{}
	if all, err := s.tasks.All(); err == nil {
		for _, task := range all {
			if !task.Archived && !task.Completed {
				counts.Inbox++
			}
		}
		counts.Tasks = len(all)
	}
	if s.contacts != nil {
		if n, err := s.contacts.CountContacts(); err == nil {
			counts.Contacts = n
		}
	}
	counts.Workflows = len(s.workflowThreads())
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
