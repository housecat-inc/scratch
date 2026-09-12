package inbox

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
)

func (s *Server) ConfigurePages(store db.PageStore) { s.pages = store }

func (s *Server) handlePages(w http.ResponseWriter, r *http.Request) {
	p, err := s.props("pages", "all", ui.InboxSelection{})
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, ui.PagesHome(p))
}

func (s *Server) page(w http.ResponseWriter, r *http.Request) (db.Page, bool) {
	if s.pages == nil {
		http.NotFound(w, r)
		return db.Page{}, false
	}
	p, err := s.pages.GetPage(r.PathValue("page"))
	if errors.Is(err, sql.ErrNoRows) {
		http.NotFound(w, r)
		return p, false
	}
	if err != nil {
		s.fail(w, err)
		return p, false
	}
	return p, true
}

func (s *Server) handlePage(w http.ResponseWriter, r *http.Request) {
	page, ok := s.page(w, r)
	if !ok {
		return
	}
	p, err := s.props("pages", "all", ui.InboxSelection{})
	if err != nil {
		s.fail(w, err)
		return
	}
	if page.Href() != "/pages/"+page.ID {
		http.Redirect(w, r, page.Href(), http.StatusSeeOther)
		return
	}
	p.Page = &page
	s.render(w, r, ui.PageReader(p, page))
}

func (s *Server) handlePagePin(w http.ResponseWriter, r *http.Request) {
	page, ok := s.page(w, r)
	if !ok {
		return
	}
	if err := s.pages.SetPagePinned(page.ID, r.FormValue("pinned") == "true"); err != nil {
		s.fail(w, err)
		return
	}
	back := "/pages"
	if r.FormValue("back") == "page" {
		back = page.Href()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (s *Server) handlePageChat(w http.ResponseWriter, r *http.Request) {
	s.pageMu.Lock()
	defer s.pageMu.Unlock()
	page, ok := s.page(w, r)
	if !ok {
		return
	}
	if page.ThreadID == 0 {
		thread, err := s.chat.CreateThread("", "Edit "+page.Title)
		if err != nil {
			s.fail(w, err)
			return
		}
		if err := s.pages.SetPageThread(page.ID, thread.ID); err != nil {
			_ = s.chat.DeleteThread(thread.ID)
			s.fail(w, err)
			return
		}
		page.ThreadID = thread.ID
	}
	http.Redirect(w, r, "/inbox/chats/"+strconv.FormatInt(page.ThreadID, 10), http.StatusSeeOther)
}

func (s *Server) PageNavigation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.pages != nil && r.Method == http.MethodGet {
			pages, err := s.pages.ListPages()
			if err != nil {
				s.fail(w, err)
				return
			}
			ctx := ui.WithPageNavigation(r.Context(), pages)
			ctx = ui.WithChatSidebar(ctx, s.chat.SidebarProps)
			r = r.WithContext(ui.WithCurrentPage(ctx, pages, r.URL.RequestURI()))
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	if s.pages != nil {
		pages, err := s.pages.ListPages()
		if err != nil {
			s.fail(w, err)
			return
		}
		for _, page := range pages {
			if page.Home {
				http.Redirect(w, r, page.Href(), http.StatusSeeOther)
				return
			}
		}
	}
	http.Redirect(w, r, "/getting-started", http.StatusSeeOther)
}

func (s *Server) handlePageHome(w http.ResponseWriter, r *http.Request) {
	page, ok := s.page(w, r)
	if !ok {
		return
	}
	if err := s.pages.SetPageHome(page.ID); err != nil {
		s.fail(w, err)
		return
	}
	back := "/pages"
	if r.FormValue("back") == "page" {
		back = page.Href()
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}
