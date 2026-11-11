package appserver_test

import (
	"context"
	"encoding/json"
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

func start(t *testing.T) *appserver.Client {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, err := appserver.Start(ctx, appservertest.Command(), appserver.ClientInfo{Name: "scratch", Title: "scratch", Version: "test"})
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Close() })
	return c
}

func TestClient(t *testing.T) {
	t.Run("Call", func(t *testing.T) {
		t.Run("answers server requests with an error", func(t *testing.T) {
			r := require.New(t)
			c := start(t)

			answered := make(chan struct{}, 1)
			defer c.Subscribe(func(n appserver.Notification) {
				if n.Method == "test/serverRequestAnswered" {
					answered <- struct{}{}
				}
			})()
			r.NoError(c.Call(context.Background(), "test/ping", nil, nil))
			select {
			case <-answered:
			case <-time.After(5 * time.Second):
				t.Fatal("server request not answered")
			}
		})

		t.Run("fails after close", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			c := start(t)

			r.NoError(c.Close())
			a.Error(c.Call(context.Background(), appserver.MethodAccountLoginStart, appserver.LoginParams{Type: appserver.LoginTypeChatGPT}, nil))
		})

		t.Run("returns rpc errors", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			c := start(t)

			err := c.Call(context.Background(), "nope", nil, nil)
			var rpcErr *appserver.RPCError
			r.ErrorAs(err, &rpcErr)
			a.Equal(-32601, rpcErr.Code)
		})
	})

	t.Run("LoginCancel", func(t *testing.T) {
		t.Run("emits a failed login completed notification", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			c := start(t)

			got := make(chan appserver.LoginCompleted, 1)
			defer c.Subscribe(func(n appserver.Notification) {
				if n.Method != appserver.MethodAccountLoginCompleted {
					return
				}
				var lc appserver.LoginCompleted
				_ = json.Unmarshal(n.Params, &lc)
				got <- lc
			})()
			r.NoError(c.LoginCancel(context.Background(), appservertest.LoginID))
			select {
			case lc := <-got:
				a.False(lc.Success)
				r.NotNil(lc.Error)
				a.Equal("cancelled", *lc.Error)
			case <-time.After(5 * time.Second):
				t.Fatal("no login completed notification")
			}
		})
	})

	t.Run("LoginStart", func(t *testing.T) {
		t.Run("returns an auth url for browser login", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			c := start(t)

			res, err := c.LoginStart(context.Background(), appserver.LoginParams{Type: appserver.LoginTypeChatGPT})
			r.NoError(err)
			a.Equal(appservertest.LoginID, res.LoginID)
			a.Contains(res.AuthURL, "redirect_uri=")
		})

		t.Run("returns a user code for device code login", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)
			c := start(t)

			res, err := c.LoginStart(context.Background(), appserver.LoginParams{Type: appserver.LoginTypeDeviceCode})
			r.NoError(err)
			a.Equal(appservertest.DeviceCode, res.UserCode)
			a.Equal(appservertest.DeviceURL, res.VerificationURL)
		})
	})
}
