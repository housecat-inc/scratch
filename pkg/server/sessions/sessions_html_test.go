package sessions

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
)

func TestSetupPageHTML(t *testing.T) {
	type step = testkit.Step[*testkit.HTML]

	absent := testkit.AbsentStep[*testkit.HTML]
	present := testkit.PresentStep[*testkit.HTML]
	textContains := testkit.TextContainsStep[*testkit.HTML]

	missing := func(sub string) step {
		return func(t *testing.T, h *testkit.HTML) {
			t.Helper()
			h.A.NotContains(h.Doc.Find("body").Text(), sub)
		}
	}

	cards := []step{
		present("#card-install"),
		present("#card-login"),
		present("#card-configure"),
		present("#card-codex-install"),
		present("#card-codex-login"),
	}

	testkit.RunCases(t, []testkit.Case[*testkit.HTML]{
		{
			Assert: append([]step{
				textContains("body", "scratch"),
				textContains("body", "Claude Code"),
				textContains("body", "Codex"),
				textContains("body", "Workspace"),
				textContains("body", "Install Claude Code"),
				textContains("body", "Install Codex"),
				textContains("body", "Configure"),
				present(`[hx-post="/install"]`),
				absent(`[hx-post="/configure"]`),
				absent(`[hx-post="/login"]`),
			}, cards...),
			Data: fakeDeps{},
			Name: "fresh state groups claude and codex steps",
			Path: "/setup",
		},
		{
			Assert: []step{
				present(`[hx-post="/install"]`),
				textContains("body", "Update"),
				textContains("body", "claude v2.1.139"),
				present(`[hx-post="/login"]`),
				absent(`[hx-post="/configure"]`),
			},
			Data: fakeDeps{claudeVersion: "2.1.139", installed: true},
			Name: "installed shows update button and version, activates sign in",
			Path: "/setup",
		},
		{
			Assert: []step{
				present("#card-codex-install"),
				textContains("body", "codex v0.5.2"),
			},
			Data: fakeDeps{codexInstalled: true, codexVersion: "0.5.2", installed: true},
			Name: "codex install row reports version and marks done",
			Path: "/setup",
		},
		{
			Assert: []step{
				present("#card-codex-install"),
				textContains("body", "codex CLI not installed"),
				textContains("body", "Get Codex"),
			},
			Data: fakeDeps{installed: true},
			Name: "codex not installed prompts to get codex",
			Path: "/setup",
		},
		{
			Assert: []step{
				present("#card-codex-login"),
				missing("codex login"),
			},
			Data: fakeDeps{codexAuthenticated: true, codexInstalled: true, installed: true},
			Name: "codex authenticated marks sign in done",
			Path: "/setup",
		},
		{
			Assert: []step{
				present(`[hx-post="/configure"]`),
				absent(`[hx-post="/login"]`),
			},
			Data: fakeDeps{authenticated: true, installed: true},
			Name: "authenticated checks sign in, activates configure",
			Path: "/setup",
		},
		{
			Assert: []step{
				textContains("body", "Update"),
				absent(`[hx-post="/login"]`),
				absent(`[hx-post="/configure"]`),
			},
			Data: fakeDeps{authenticated: true, configured: true, installed: true},
			Name: "all done still shows update button",
			Path: "/setup",
		},
	}, testkit.CaseRunner[*testkit.HTML]{
		Load: func(h *testkit.HTML, path string) { h.Load(path) },
		Setup: func(t *testing.T, kit *testkit.T, tc testkit.Case[*testkit.HTML]) *testkit.HTML {
			fd := tc.Data.(fakeDeps)
			s, err := NewServer(fd.deps())
			kit.R.NoError(err)
			return testkit.NewHTMLWithT(t, kit, s.Handler())
		},
	})
}
