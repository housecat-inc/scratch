package sessions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGettingStartedPage(t *testing.T) {
	for _, tc := range []struct {
		fake fakeDeps
		name string
		path string
		want []string
	}{
		{name: "welcome", path: "/getting-started", want: []string{"Welcome to Scratch", "standalone Agent Computer", "/getting-started?step=connect"}},
		{name: "unknown step returns welcome", path: "/getting-started?step=unknown", want: []string{"Welcome to Scratch"}},
		{name: "connect installed providers", path: "/getting-started?step=connect", fake: fakeDeps{installed: true, codexInstalled: true}, want: []string{"Connect your subscription", `hx-post="/login"`, `hx-post="/codex/login"`, "Do this later"}},
		{name: "connect missing providers", path: "/getting-started?step=connect", want: []string{"Install Claude Code", "Get Codex"}},
		{name: "start without subscription", path: "/getting-started?step=start", want: []string{"Chat with your agent", "Build a new workflow", "You can explore now", `href="/inbox/workflows/new"`}},
		{name: "start with codex", path: "/getting-started?step=start", fake: fakeDeps{codexAuthenticated: true}, want: []string{"/inbox/chats/new?agent=codex"}},
		{name: "setup stays separate", path: "/setup", want: []string{"/getting-started", "card-configure"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			s, err := NewServer(tc.fake.deps())
			r.NoError(err)
			response := httptest.NewRecorder()
			s.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodGet, tc.path, nil))
			r.Equal(http.StatusOK, response.Code)
			for _, want := range tc.want {
				a.Contains(response.Body.String(), want)
			}
		})
	}
}
