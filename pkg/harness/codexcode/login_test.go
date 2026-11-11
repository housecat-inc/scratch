package codexcode

import (
	"net/url"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver"
	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver/appservertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppServerHelper(t *testing.T) {
	if appservertest.Enabled() {
		appservertest.Main()
	}
}

func callbackURL(t *testing.T, l *Login, query url.Values) string {
	t.Helper()
	au, err := url.Parse(l.URL())
	require.NoError(t, err)
	u, err := url.Parse(au.Query().Get("redirect_uri"))
	require.NoError(t, err)
	u.RawQuery = query.Encode()
	return u.String()
}

func waitDone(l *Login) bool {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if l.Done() {
			return true
		}
		time.Sleep(20 * time.Millisecond)
	}
	return false
}

func TestBrowserLogin(t *testing.T) {
	tests := []struct {
		name     string
		paste    func(t *testing.T, l *Login) string
		wantDone bool
		wantErr  string
	}{
		{
			name: "valid callback completes sign in",
			paste: func(t *testing.T, l *Login) string {
				return callbackURL(t, l, url.Values{"code": {appservertest.GoodCode}, "state": {appservertest.State}})
			},
			wantDone: true,
		},
		{
			name: "rejected code surfaces codex error",
			paste: func(t *testing.T, l *Login) string {
				return callbackURL(t, l, url.Values{"code": {"bad"}, "state": {appservertest.State}})
			},
			wantDone: true,
			wantErr:  "token exchange failed",
		},
		{
			name:    "missing code is rejected locally",
			paste:   func(t *testing.T, l *Login) string { return "http://localhost:1455/auth/callback" },
			wantErr: "paste the full URL",
		},
		{
			name:    "wrong path is rejected locally",
			paste:   func(t *testing.T, l *Login) string { return "http://localhost:1455/other?code=x&state=y" },
			wantErr: "expected a /auth/callback URL",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			l, err := StartLoginCmd(appservertest.Command(), appserver.LoginTypeChatGPT)
			r.NoError(err)
			t.Cleanup(func() { _ = l.Close() })
			a.Contains(l.URL(), "https://auth.openai.com/oauth/authorize")
			a.Empty(l.Code())

			err = l.SubmitCallback(tt.paste(t, l))
			if tt.wantErr != "" {
				a.ErrorContains(err, tt.wantErr)
			} else {
				a.NoError(err)
			}
			a.Equal(tt.wantDone, l.Done())
		})
	}
}

func TestDeviceLogin(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	l, err := StartLoginCmd(appservertest.Command(appservertest.EnvComplete+"=1"), appserver.LoginTypeDeviceCode)
	r.NoError(err)
	t.Cleanup(func() { _ = l.Close() })

	a.Equal(appservertest.DeviceURL, l.URL())
	a.Equal(appservertest.DeviceCode, l.Code())
	a.ErrorContains(l.SubmitCallback("http://localhost:1455/auth/callback?code=x&state=y"), "not a browser sign in")
	a.True(waitDone(l))
	a.NoError(l.Err())
}

func TestLoginCloseCancels(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	l, err := StartLoginCmd(appservertest.Command(), appserver.LoginTypeChatGPT)
	r.NoError(err)
	r.NoError(l.Close())
	a.True(waitDone(l))
	a.Error(l.Err())
}
