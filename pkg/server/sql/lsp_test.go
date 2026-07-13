package sql

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func TestLSPBridge(t *testing.T) {
	r := require.New(t)

	src, err := exec.LookPath("sqls")
	if err != nil {
		t.Skip("sqls not installed; skipping LSP bridge test")
	}

	binDir := t.TempDir()
	copyExec(t, src, filepath.Join(binDir, "sqls"))

	s := newServer(t, seedDB(t))
	s.deps.BinDir = binDir

	srv := httptest.NewServer(http.StripPrefix("/sql", s.Handler()))
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/sql/lsp"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			t.Logf("handshake status=%d body=%q", resp.StatusCode, body)
		}
	}
	r.NoError(err)
	defer conn.Close()

	send(t, conn, map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "initialize",
		"params":  map[string]any{"rootUri": "file:///", "capabilities": map[string]any{}},
	})

	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var reply map[string]any
	for {
		_, data, err := conn.ReadMessage()
		r.NoError(err)
		r.NoError(json.Unmarshal(data, &reply))
		if id, ok := reply["id"].(float64); ok && id == 1 {
			break
		}
	}

	result, ok := reply["result"].(map[string]any)
	r.True(ok, "initialize result")
	caps, ok := result["capabilities"].(map[string]any)
	r.True(ok, "server capabilities")
	r.Contains(caps, "completionProvider")
}

func send(t *testing.T, conn *websocket.Conn, msg map[string]any) {
	t.Helper()
	data, err := json.Marshal(msg)
	require.NoError(t, err)
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, data))
}

func copyExec(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	require.NoError(t, err)
	_, err = io.Copy(out, in)
	require.NoError(t, err)
	require.NoError(t, out.Close())
}
