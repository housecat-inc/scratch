package sessions

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	tk "github.com/housecat-inc/scratch/testkit/v2"
)

func TestSetupPageHTML(t *testing.T) {
	type H = *testkit.HTML

	serve := func(fd fakeDeps) func(*tk.T) H {
		return func(t *tk.T) H {
			srv, err := NewServer(fd.deps())
			t.R.NoError(err)
			return testkit.NewHTML(t.T, srv.Handler())
		}
	}

	s := tk.Steps[H]{}
	missing := func(sub string) tk.Step[H] {
		return func(t *tk.T, h H) {
			t.Helper()
			t.A.NotContains(h.Doc.Find("body").Text(), sub)
		}
	}

	cards := []tk.Step[H]{
		s.Present("#card-install"),
		s.Present("#card-login"),
		s.Present("#card-configure"),
		s.Present("#card-codex-install"),
		s.Present("#card-codex-login"),
	}

	tk.RunSteps(t, []tk.Scenario[H]{
		{
			Name: "fresh state groups claude and codex steps",
			Steps: append([]tk.Step[H]{
				s.Visit("/setup"),
				s.Text("body", "scratch"),
				s.Text("body", "Claude Code"),
				s.Text("body", "Codex"),
				s.Text("body", "Workspace"),
				s.Text("body", "Install Claude Code"),
				s.Text("body", "Install Codex"),
				s.Text("body", "Configure"),
				s.Present(`[hx-post="/install"]`),
				s.Absent(`[hx-post="/configure"]`),
				s.Absent(`[hx-post="/login"]`),
			}, cards...),
		},
		{
			Name:  "installed shows update button and version, activates sign in",
			Setup: serve(fakeDeps{claudeVersion: "2.1.139", installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Present(`[hx-post="/install"]`),
				s.Text("body", "Update"),
				s.Text("body", "claude v2.1.139"),
				s.Present(`[hx-post="/login"]`),
				s.Absent(`[hx-post="/configure"]`),
			},
		},
		{
			Name:  "codex install row reports version and marks done",
			Setup: serve(fakeDeps{codexInstalled: true, codexVersion: "0.5.2", installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Present("#card-codex-install"),
				s.Text("body", "codex v0.5.2"),
			},
		},
		{
			Name:  "codex not installed prompts to get codex",
			Setup: serve(fakeDeps{installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Present("#card-codex-install"),
				s.Text("body", "codex CLI not installed"),
				s.Text("body", "Get Codex"),
			},
		},
		{
			Name:  "codex authenticated marks sign in done",
			Setup: serve(fakeDeps{codexAuthenticated: true, codexInstalled: true, installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Present("#card-codex-login"),
				missing("codex login"),
			},
		},
		{
			Name:  "authenticated checks sign in, activates configure",
			Setup: serve(fakeDeps{authenticated: true, installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Present(`[hx-post="/configure"]`),
				s.Absent(`[hx-post="/login"]`),
			},
		},
		{
			Name:  "all done still shows update button",
			Setup: serve(fakeDeps{authenticated: true, configured: true, installed: true}),
			Steps: []tk.Step[H]{
				s.Visit("/setup"),
				s.Text("body", "Update"),
				s.Absent(`[hx-post="/login"]`),
				s.Absent(`[hx-post="/configure"]`),
			},
		},
	}, serve(fakeDeps{}))
}
