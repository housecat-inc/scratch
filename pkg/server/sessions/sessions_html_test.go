package sessions

import (
	"testing"

	"github.com/housecat-inc/scratch/testkit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSessionsHTML(t *testing.T) {
	t.Run("setup page", func(t *testing.T) {
		t.Run("groups claude and codex steps on a fresh machine", func(t *testing.T) {
			h := setupPage(t, fakeDeps{})
			h.ElementTextContains("body", "scratch")
			h.ElementTextContains("body", "Claude Code")
			h.ElementTextContains("body", "Codex")
			h.ElementTextContains("body", "Workspace")
			h.ElementTextContains("body", "Install Claude Code")
			h.ElementTextContains("body", "Install Codex")
			h.ElementTextContains("body", "Configure")
			h.ElementPresent(`[hx-post="/install"]`)
			h.ElementAbsent(`[hx-post="/configure"]`)
			h.ElementAbsent(`[hx-post="/login"]`)
			for _, card := range []string{"#card-install", "#card-login", "#card-configure", "#card-codex-install", "#card-codex-login"} {
				h.ElementPresent(card)
			}
		})
		t.Run("shows the claude version and update button once installed and activates sign in", func(t *testing.T) {
			h := setupPage(t, fakeDeps{claudeVersion: "2.1.139", installed: true})
			h.ElementPresent(`[hx-post="/install"]`)
			h.ElementTextContains("body", "Update")
			h.ElementTextContains("body", "claude v2.1.139")
			h.ElementPresent(`[hx-post="/login"]`)
			h.ElementAbsent(`[hx-post="/configure"]`)
		})
		t.Run("reports the installed codex version", func(t *testing.T) {
			h := setupPage(t, fakeDeps{codexInstalled: true, codexVersion: "0.5.2", installed: true})
			h.ElementPresent("#card-codex-install")
			h.ElementTextContains("body", "codex v0.5.2")
		})
		t.Run("prompts to get codex when it is not installed", func(t *testing.T) {
			h := setupPage(t, fakeDeps{installed: true})
			h.ElementPresent("#card-codex-install")
			h.ElementTextContains("body", "codex CLI not installed")
			h.ElementTextContains("body", "Get Codex")
		})
		t.Run("marks codex sign in done when authenticated", func(t *testing.T) {
			a := assert.New(t)
			h := setupPage(t, fakeDeps{codexAuthenticated: true, codexInstalled: true, installed: true})
			h.ElementPresent("#card-codex-login")
			a.NotContains(h.Doc.Find("body").Text(), "codex login")
		})
		t.Run("checks sign in and activates configure when authenticated", func(t *testing.T) {
			h := setupPage(t, fakeDeps{authenticated: true, installed: true})
			h.ElementPresent(`[hx-post="/configure"]`)
			h.ElementAbsent(`[hx-post="/login"]`)
		})
		t.Run("still offers the update button when every step is done", func(t *testing.T) {
			h := setupPage(t, fakeDeps{authenticated: true, configured: true, installed: true})
			h.ElementTextContains("body", "Update")
			h.ElementAbsent(`[hx-post="/login"]`)
			h.ElementAbsent(`[hx-post="/configure"]`)
		})
	})
}

func setupPage(t *testing.T, fd fakeDeps) *testkit.HTML {
	t.Helper()
	srv, err := NewServer(fd.deps())
	require.New(t).NoError(err)
	h := testkit.NewHTML(t, srv.Handler())
	h.Load("/setup")
	return h
}
