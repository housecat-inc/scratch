package updates

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdates(t *testing.T) {
	for _, tt := range []struct {
		name         string
		pending      bool
		restartError bool
		token        string
		origin       string
		want         int
		calls        int
	}{
		{name: "unchanged", token: "boot", want: http.StatusConflict},
		{name: "pending", pending: true, token: "boot", want: http.StatusAccepted, calls: 1},
		{name: "missing token", pending: true, want: http.StatusForbidden},
		{name: "stale token", pending: true, token: "old", want: http.StatusForbidden},
		{name: "cross origin", pending: true, token: "boot", origin: "https://other.example", want: http.StatusForbidden},
		{name: "restart failure", pending: true, restartError: true, token: "boot", want: http.StatusInternalServerError, calls: 1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			path := filepath.Join(t.TempDir(), "scratch")
			r.NoError(os.WriteFile(path, []byte("old"), 0700))
			running, err := os.Stat(path)
			r.NoError(err)
			calls := 0
			s := &Server{instance: "boot", path: path, running: running, restart: func() error {
				calls++
				if tt.restartError {
					return errors.New("failed")
				}
				return nil
			}}
			if tt.pending {
				next := path + ".next"
				r.NoError(os.WriteFile(next, []byte("new"), 0700))
				r.NoError(os.Rename(next, path))
			}
			a.Equal(tt.pending, s.pending())
			handler := s.Handler()
			status := httptest.NewRecorder()
			handler.ServeHTTP(status, httptest.NewRequest(http.MethodGet, "/app/updates", nil))
			a.Equal(http.StatusOK, status.Code)
			a.Equal("no-store", status.Header().Get("Cache-Control"))
			a.Zero(calls)
			req := httptest.NewRequest(http.MethodPost, "/app/restart", nil)
			req.Header.Set("X-Scratch-Instance", tt.token)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, req)
			a.Equal(tt.want, response.Code)
			a.Equal(tt.calls, calls)
			a.Equal(tt.want == http.StatusAccepted, s.restarting)
			if s.restarting {
				handler.ServeHTTP(httptest.NewRecorder(), req)
				a.Equal(1, calls)
			}
			s.restart = nil
			a.False(s.pending())
		})
	}
}
