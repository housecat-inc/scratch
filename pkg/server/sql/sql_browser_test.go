package sql

import (
	"log/slog"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/testkit"
)

func TestSQLBrowser(t *testing.T) {
	t.Run("mounts a single codemirror editor beside saved queries and schema", func(t *testing.T) {
		h := sqlBrowser(t)
		h.Load("/sql/")
		h.ElementTextContains(".mail-brand", "scratch")
		h.ElementTextContains(".mail-labels", "SQL")
		h.ElementAttributeContains(`a[href="/sql/"]`, "class", "active")
		h.ElementTextContains("#sql-query-list", "tasks")
		h.ElementTextContains(".tree-pane", "widgets")
		h.ElementVisible(".cm-editor")
		h.ElementHidden("#sql-editor")
		savedQueryTrashAligned(h)
	})
	t.Run("runs a table query from the sidebar", func(t *testing.T) {
		h := sqlBrowser(t)
		h.Load("/sql/")
		h.Click("[data-sql]")
		h.ElementTextContains("#sql-results", "alpha")
		h.ElementTextContains("#sql-results", "bravo")
	})
	t.Run("shows shared shell counts and opens chat", func(t *testing.T) {
		h := sqlBrowser(t)
		h.Load("/sql/")
		h.Click("[data-new-chat]")
		h.ElementTextContains(`a[href="/"] .gm-label-count`, "2")
		h.ElementTextContains(`a[href="/inbox/chats"] .gm-label-count`, "3")
		h.ElementTextContains(`a[href="/inbox/tasks"] .gm-label-count`, "4")
		h.ElementVisible(".mail-footer [data-chat-input]")
		h.ElementVisible("#chat-rail-panel [data-chat-input]")
	})
}

func sqlBrowser(t *testing.T) *testkit.Harness {
	t.Helper()
	kit := testkit.New(t)
	store, err := db.New(filepath.Join(t.TempDir(), "scratch.db"))
	kit.R.NoError(err)
	t.Cleanup(func() { store.Close() })
	_, err = store.SaveSQLQuery("tasks", "SELECT *\nFROM tasks;")
	kit.R.NoError(err)
	chatSvc := chat.NewService(store, chat.EchoAgent{}, slog.Default())
	t.Cleanup(chatSvc.Close)
	shell := func() ui.ToolShellProps {
		return ui.ToolShellProps{
			Counts: ui.InboxCounts{
				Chats:     3,
				Inbox:     2,
				Starred:   1,
				Tasks:     4,
				Workflows: 5,
			},
		}
	}
	s, err := NewServer(Deps{DefaultPath: seedDB(t), Shell: shell, Store: store})
	kit.R.NoError(err)
	mux := http.NewServeMux()
	mux.Handle("/chat/", chat.NewServer(chatSvc, slog.Default()).Handler())
	mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.Handle("/sql/", http.StripPrefix("/sql", s.Handler()))
	return testkit.NewHarnessWithT(t, kit, mux)
}

func savedQueryTrashAligned(h *testkit.Harness) {
	h.R.Eventually(func() bool {
		result, err := h.Page.Eval(`() => {
			const row = document.querySelector("#sql-query-list li");
			const icon = document.querySelector('#sql-query-list [aria-label="Delete query"] svg');
			if (!row || !icon) return false;
			const iconRect = icon.getBoundingClientRect();
			const rowRect = row.getBoundingClientRect();
			const iconCenter = iconRect.top + iconRect.height / 2;
			const rowCenter = rowRect.top + rowRect.height / 2;
			return iconRect.width > 0 && iconRect.width <= 20 && iconRect.height > 0 && iconRect.height <= 20 && Math.abs(rowCenter - iconCenter) <= 1;
		}`)
		return err == nil && result.Value.Bool()
	}, 5*time.Second, 50*time.Millisecond)
}
