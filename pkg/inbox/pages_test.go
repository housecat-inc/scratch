package inbox

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPages(t *testing.T) {
	for _, id := range []string{"example"} {
		t.Run(id, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			store, err := db.New(":memory:")
			r.NoError(err)
			t.Cleanup(func() { store.Close() })
			chats := chat.NewService(store, chat.EchoAgent{}, nil)
			t.Cleanup(chats.Close)
			server := NewServer(todo.NewService(store), chats, nil)
			server.ConfigurePages(store)
			handler := server.Handler()
			request := func(method, path, body string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, path, strings.NewReader(body))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				return rec
			}
			a.Equal("/getting-started", request("GET", "/", "").Header().Get("Location"))
			a.Equal(http.StatusSeeOther, request("POST", "/pages/"+id+"/home", "").Code)
			a.Equal("/pages/"+id, request("GET", "/", "").Header().Get("Location"))
			a.Equal(http.StatusNotFound, request("POST", "/pages/missing/home", "").Code)
			a.Equal("/pages/"+id, request("GET", "/", "").Header().Get("Location"))
			a.Equal(http.StatusSeeOther, request("POST", "/pages/getting-started/home", "").Code)
			a.Equal("/getting-started", request("GET", "/", "").Header().Get("Location"))
			home := request("GET", "/pages", "")
			a.Equal(http.StatusOK, home.Code)
			a.Contains(home.Body.String(), "Example Page")
			a.NotContains(home.Body.String(), "Project Board")
			a.Contains(home.Body.String(), `/pages/example`)
			a.NotContains(home.Body.String(), `/pages/test123`)
			a.NotContains(home.Body.String(), `id="todays-news-title"`)
			inbox := request("GET", "/inbox", "")
			a.Equal(http.StatusOK, inbox.Code)
			a.NotContains(inbox.Body.String(), `id="todays-news-title"`)
			page := request("GET", "/pages/"+id, "")
			a.Equal(http.StatusOK, page.Code)
			a.Contains(page.Body.String(), "Edit with agent")
			a.Contains(page.Body.String(), "Make this page yours")
			a.NotContains(page.Body.String(), "<iframe")
			pin := request("POST", "/pages/"+id+"/pin", "pinned=false")
			a.Equal(http.StatusSeeOther, pin.Code)
			saved, err := store.GetPage(id)
			r.NoError(err)
			a.False(saved.Pinned)
			a.Equal(http.StatusSeeOther, request("POST", "/pages/"+id+"/pin", "pinned=true").Code)
			saved, err = store.GetPage(id)
			r.NoError(err)
			a.True(saved.Pinned)
			props, err := server.props("pages", "all", ui.InboxSelection{})
			r.NoError(err)
			a.Len(props.Pages, 13)
			first := request("POST", "/pages/"+id+"/chat", "")
			r.Equal(http.StatusSeeOther, first.Code)
			second := request("POST", "/pages/"+id+"/chat", "")
			a.Equal(first.Header().Get("Location"), second.Header().Get("Location"))
			threads, err := chats.Threads()
			r.NoError(err)
			a.Len(threads, 1)
			reader := request("GET", first.Header().Get("Location"), "")
			a.Equal(http.StatusOK, reader.Code)
			a.Contains(reader.Body.String(), "Describe what you’d like to change.")
			saved, err = store.GetPage(id)
			r.NoError(err)
			r.NotZero(saved.ThreadID)
			r.NoError(chats.DeleteThread(saved.ThreadID))
			next := request("POST", "/pages/"+id+"/chat", "")
			a.Equal(http.StatusSeeOther, next.Code)
			a.Equal(http.StatusNotFound, request("GET", "/pages/missing", "").Code)
			a.Equal(http.StatusNotFound, request("POST", "/pages/missing/chat", "").Code)
		})
	}
}

func TestBuiltinPageActions(t *testing.T) {
	store, err := db.New(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	server := NewServer(nil, nil, nil)
	server.ConfigurePages(store)
	handler := server.Handler()
	for _, id := range []string{"agents", "chats", "code", "files", "inbox", "pages", "sessions", "setup", "starred", "tasks", "workflows"} {
		t.Run(id, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			page, err := store.GetPage(id)
			r.NoError(err)
			for _, action := range []string{"home", "pin"} {
				req := httptest.NewRequest("POST", "/pages/"+id+"/"+action, strings.NewReader("back=page&pinned=false"))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, req)
				a.Equal(http.StatusSeeOther, rec.Code)
				a.Equal(page.Href(), rec.Header().Get("Location"))
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))
			a.Equal(page.Href(), rec.Header().Get("Location"))
			saved, err := store.GetPage(id)
			r.NoError(err)
			a.True(saved.Home)
			a.False(saved.Pinned)
		})
	}
}

func TestPageSidebarRenderedOnReopen(t *testing.T) {
	for _, path := range []string{"/pages", "/pages/example", "/inbox/tasks"} {
		t.Run(path, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			store, err := db.New(":memory:")
			r.NoError(err)
			t.Cleanup(func() { store.Close() })
			chats := chat.NewService(store, chat.EchoAgent{}, nil)
			t.Cleanup(chats.Close)
			thread, err := chats.CreateThread("", "Persistent sidebar chat")
			r.NoError(err)
			server := NewServer(todo.NewService(store), chats, nil)
			server.ConfigurePages(store)
			handler := server.PageNavigation(server.Handler())
			for range 2 {
				response := httptest.NewRecorder()
				handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
				r.Equal(http.StatusOK, response.Code)
				a.Contains(response.Body.String(), "Persistent sidebar chat")
				a.NotContains(response.Body.String(), `hx-get="/chat/sidebar" hx-trigger="load"`)
			}
			r.NoError(chats.DeleteThread(thread.ID))
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
			a.NotContains(response.Body.String(), "Persistent sidebar chat")
		})
	}
}
