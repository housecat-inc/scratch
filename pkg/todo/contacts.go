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
	chatOptions, chatLabel := s.chatActions()
	props := ui.ContactsProps{
		ChatLabel:   chatLabel,
		ChatOptions: chatOptions,
		Contacts:    items,
		Counts:      s.navCounts(),
		Page:        page,
		PerPage:     contactsPerPage,
		Total:       total,
	}
	if r.Header.Get("HX-Request") != "" {
		s.render(w, r, ui.ContactsTable(props))
		return
	}
	s.render(w, r, ui.ContactsPage(props))
}

func (s *WebServer) handleContact(w http.ResponseWriter, r *http.Request) {
	if s.contacts == nil {
		http.NotFound(w, r)
		return
	}
	id, ok := taskPathID(w, r)
	if !ok {
		return
	}
	contact, err := s.contacts.GetContact(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	emails, err := s.contacts.ListContactEmails(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	notes, err := s.contacts.ListContactNotes(id)
	if err != nil {
		s.fail(w, err)
		return
	}
	noteViews := make([]ui.ContactNoteView, 0, len(notes))
	for _, note := range notes {
		noteViews = append(noteViews, ui.ContactNoteView{
			Body: note.Body,
			When: note.CreatedAt.Time.Format("Jan 2, 2006 3:04 PM"),
		})
	}
	chatOptions, chatLabel := s.chatActions()
	s.render(w, r, ui.ContactDetailPage(ui.ContactDetailProps{
		ChatLabel:   chatLabel,
		ChatOptions: chatOptions,
		Contact:     contact,
		Counts:      s.navCounts(),
		Emails:      emails,
		Notes:       noteViews,
	}))
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
