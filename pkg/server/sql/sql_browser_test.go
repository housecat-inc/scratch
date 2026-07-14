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
	"github.com/stretchr/testify/require"
)

func TestSQLBrowser(t *testing.T) {
	testkit.RunBrowserCases(t, []testkit.BrowserCase[*testkit.Harness]{
		{
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](".mail-brand", "scratch"),
				testkit.TextContainsStep[*testkit.Harness](".mail-labels", "SQL"),
				testkit.ClassContainsStep[*testkit.Harness](`a[href="/sql/"]`, "active"),
				testkit.TextContainsStep[*testkit.Harness]("#sql-query-list", "tasks"),
				testkit.TextContainsStep[*testkit.Harness](".tree-pane", "widgets"),
				testkit.VisibleStep[*testkit.Harness](".cm-editor"),
				func(t *testing.T, h *testkit.Harness) { h.ElementHidden("#sql-editor") },
				savedQueryTrashAlignedStep(),
			},
			Name: "mounts a single codemirror editor",
			Path: "/sql/",
		},
		{
			Act: []testkit.BrowserStep[*testkit.Harness]{
				testkit.ClickStep[*testkit.Harness]("[data-sql]"),
			},
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness]("#sql-results", "alpha"),
				testkit.TextContainsStep[*testkit.Harness]("#sql-results", "bravo"),
			},
			Name: "runs table query from sidebar",
			Path: "/sql/",
		},
		{
			Act: []testkit.BrowserStep[*testkit.Harness]{
				testkit.ClickStep[*testkit.Harness]("[data-new-chat]"),
			},
			Assert: []testkit.BrowserStep[*testkit.Harness]{
				testkit.TextContainsStep[*testkit.Harness](`a[href="/"] .gm-label-count`, "2"),
				testkit.TextContainsStep[*testkit.Harness](`a[href="/inbox/chats"] .gm-label-count`, "3"),
				testkit.TextContainsStep[*testkit.Harness](`a[href="/inbox/tasks"] .gm-label-count`, "4"),
				testkit.VisibleStep[*testkit.Harness](".mail-footer [data-chat-input]"),
				testkit.VisibleStep[*testkit.Harness]("#floating-chat [data-chat-input]"),
			},
			Name: "shows shared shell counts and opens chat",
			Path: "/sql/",
		},
	}, testkit.BrowserCaseRunner[*testkit.Harness]{
		Load: func(h *testkit.Harness, path string) {
			h.Load(path)
		},
		Setup: func(t *testing.T, kit *testkit.T, _ testkit.BrowserCase[*testkit.Harness]) *testkit.Harness {
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
			require.NoError(t, err)
			mux := http.NewServeMux()
			mux.Handle("/chat/", chat.NewServer(chatSvc, slog.Default()).Handler())
			mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			mux.Handle("/sql/", http.StripPrefix("/sql", s.Handler()))
			return testkit.NewHarnessWithT(t, kit, mux)
		},
	})
}

func savedQueryTrashAlignedStep() testkit.BrowserStep[*testkit.Harness] {
	return func(t *testing.T, h *testkit.Harness) {
		t.Helper()
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
}
