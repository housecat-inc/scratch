package appserver_test

import (
	"context"
	"encoding/json"
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
	tests := []struct {
		name string
		run  func(t *testing.T, c *appserver.Client)
	}{
		{
			name: "browser login returns auth url",
			run: func(t *testing.T, c *appserver.Client) {
				a := assert.New(t)
				res, err := c.LoginStart(context.Background(), appserver.LoginParams{Type: appserver.LoginTypeChatGPT})
				require.NoError(t, err)
				a.Equal(appservertest.LoginID, res.LoginID)
				a.Contains(res.AuthURL, "redirect_uri=")
			},
		},
		{
			name: "cancel emits login completed",
			run: func(t *testing.T, c *appserver.Client) {
				a := assert.New(t)
				got := make(chan appserver.LoginCompleted, 1)
				defer c.Subscribe(func(n appserver.Notification) {
					if n.Method != appserver.MethodAccountLoginCompleted {
						return
					}
					var lc appserver.LoginCompleted
					_ = json.Unmarshal(n.Params, &lc)
					got <- lc
				})()
				require.NoError(t, c.LoginCancel(context.Background(), appservertest.LoginID))
				select {
				case lc := <-got:
					a.False(lc.Success)
					require.NotNil(t, lc.Error)
					a.Equal("cancelled", *lc.Error)
				case <-time.After(5 * time.Second):
					t.Fatal("no login completed notification")
				}
			},
		},
		{
			name: "device code login returns user code",
			run: func(t *testing.T, c *appserver.Client) {
				a := assert.New(t)
				res, err := c.LoginStart(context.Background(), appserver.LoginParams{Type: appserver.LoginTypeDeviceCode})
				require.NoError(t, err)
				a.Equal(appservertest.DeviceCode, res.UserCode)
				a.Equal(appservertest.DeviceURL, res.VerificationURL)
			},
		},
		{
			name: "rpc error is returned",
			run: func(t *testing.T, c *appserver.Client) {
				a := assert.New(t)
				err := c.Call(context.Background(), "nope", nil, nil)
				var rpcErr *appserver.RPCError
				a.ErrorAs(err, &rpcErr)
				a.Equal(-32601, rpcErr.Code)
			},
		},
		{
			name: "server requests are rejected",
			run: func(t *testing.T, c *appserver.Client) {
				answered := make(chan struct{}, 1)
				defer c.Subscribe(func(n appserver.Notification) {
					if n.Method == "test/serverRequestAnswered" {
						answered <- struct{}{}
					}
				})()
				require.NoError(t, c.Call(context.Background(), "test/ping", nil, nil))
				select {
				case <-answered:
				case <-time.After(5 * time.Second):
					t.Fatal("server request not answered")
				}
			},
		},
		{
			name: "calls fail after close",
			run: func(t *testing.T, c *appserver.Client) {
				a := assert.New(t)
				require.NoError(t, c.Close())
				a.Error(c.Call(context.Background(), appserver.MethodAccountLoginStart, appserver.LoginParams{Type: appserver.LoginTypeChatGPT}, nil))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t, start(t))
		})
	}
}
