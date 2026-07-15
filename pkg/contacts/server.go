package contacts

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const searchLimit = 20

type Server struct {
	store db.ContactStore
}

func NewServer(store db.ContactStore) *Server {
	return &Server{store: store}
}

func (s *Server) RegisterAPI(mux *http.ServeMux) {
	mux.HandleFunc("GET /contacts/search", s.handleSearch)
	mux.HandleFunc("POST /contacts", s.handleCreate)
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	items, err := s.matches(query)
	if err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := ui.ContactSearchResults(items, query).Render(r.Context(), w); err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
	}
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		httperr.Error(w, err, http.StatusBadRequest)
		return
	}
	name := strings.TrimSpace(r.FormValue("name"))
	if name == "" {
		http.Error(w, "name required", http.StatusBadRequest)
		return
	}
	contact, err := s.store.AddContact(name, "", "")
	if err != nil {
		httperr.Error(w, err, http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":   strconv.FormatInt(contact.ID, 10),
		"name": contact.Name,
	})
}

func (s *Server) matches(query string) ([]db.ContactListItem, error) {
	if query == "" {
		return s.store.ListContactsPage(searchLimit, 0)
	}
	return s.store.SearchContacts(query, searchLimit)
}
