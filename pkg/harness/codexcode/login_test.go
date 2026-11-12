package codexcode

import (
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver"
	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver/appservertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	if appservertest.Enabled() {
		appservertest.Main()
	}
	os.Exit(m.Run())
}

func callbackURL(t *testing.T, l *Login, query url.Values) string {
	t.Helper()
	r := require.New(t)
	au, err := url.Parse(l.URL())
	r.NoError(err)
	u, err := url.Parse(au.Query().Get("redirect_uri"))
	r.NoError(err)
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

func TestLogin(t *testing.T) {

	t.Run("when signing in with the browser", func(t *testing.T) {
		tests := []struct {
			name     string
			paste    func(t *testing.T, l *Login) string
			wantDone bool
			wantErr  string
		}{
			{
				name: "completes sign in with a valid callback",
				paste: func(t *testing.T, l *Login) string {
					return callbackURL(t, l, url.Values{"code": {appservertest.GoodCode}, "state": {appservertest.State}})
				},
				wantDone: true,
			},
			{
				name: "surfaces the codex error for a rejected code",
				paste: func(t *testing.T, l *Login) string {
					return callbackURL(t, l, url.Values{"code": {"bad"}, "state": {appservertest.State}})
				},
				wantDone: true,
				wantErr:  "token exchange failed",
			},
			{
				name:    "rejects a callback without a code locally",
				paste:   func(t *testing.T, l *Login) string { return "http://localhost:1455/auth/callback" },
				wantErr: "paste the full URL",
			},
			{
				name:    "rejects a callback on the wrong path locally",
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
	})

	t.Run("when signing in with a device code", func(t *testing.T) {
		t.Run("shows the code and completes without a callback", func(t *testing.T) {
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
		})
	})
}

func TestClose(t *testing.T) {
	t.Run("cancels a pending login", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		l, err := StartLoginCmd(appservertest.Command(), appserver.LoginTypeChatGPT)
		r.NoError(err)
		r.NoError(l.Close())
		a.True(waitDone(l))
		a.Error(l.Err())
	})
}
