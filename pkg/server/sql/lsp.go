package sql

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/gorilla/websocket"
)

var lspUpgrader = websocket.Upgrader{
	CheckOrigin: sameOrigin,
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func (s *Server) handleLSP(w http.ResponseWriter, r *http.Request) {
	path := s.resolvePath(r.URL.Query().Get("path"))
	bin, err := s.sqlsBinary(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}

	cfgPath, err := writeSqlsConfig(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer os.Remove(cfgPath)

	conn, err := lspUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "-config", cfgPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return
	}
	defer func() {
		cancel()
		_ = cmd.Wait()
	}()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		defer cancel()
		_ = pumpWSToLSP(conn, stdin)
	}()

	go func() {
		defer wg.Done()
		defer cancel()
		_ = pumpLSPToWS(stdout, conn)
	}()

	<-ctx.Done()
	stdin.Close()
	wg.Wait()
}

func writeSqlsConfig(dbPath string) (string, error) {
	abs, err := filepath.Abs(dbPath)
	if err != nil {
		return "", errors.Wrap(err, "abs db path")
	}
	dsn := "file:" + (&url.URL{Path: abs}).EscapedPath() + "?mode=ro"
	yaml := fmt.Sprintf(`lowercaseKeywords: false
connections:
  - alias: scratch
    driver: sqlite3
    dataSourceName: %s
`, dsn)
	f, err := os.CreateTemp("", "sqls-*.yml")
	if err != nil {
		return "", errors.Wrap(err, "sqls config tempfile")
	}
	defer f.Close()
	if _, err := f.WriteString(yaml); err != nil {
		return "", errors.Wrap(err, "write sqls config")
	}
	return f.Name(), nil
}

func pumpWSToLSP(conn *websocket.Conn, stdin io.WriteCloser) error {
	for {
		mt, data, err := conn.ReadMessage()
		if err != nil {
			return err
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}
		header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(data))
		if _, err := stdin.Write([]byte(header)); err != nil {
			return err
		}
		if _, err := stdin.Write(data); err != nil {
			return err
		}
	}
}

func pumpLSPToWS(stdout io.Reader, conn *websocket.Conn) error {
	br := bufio.NewReader(stdout)
	for {
		length := 0
		for {
			line, err := br.ReadString('\n')
			if err != nil {
				return err
			}
			if line == "\r\n" || line == "\n" {
				break
			}
			const key = "Content-Length:"
			if strings.HasPrefix(line, key) {
				n, err := strconv.Atoi(strings.TrimSpace(line[len(key):]))
				if err != nil {
					return errors.Wrap(err, "parse content-length")
				}
				length = n
			}
		}
		if length == 0 {
			continue
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(br, buf); err != nil {
			return err
		}
		if err := conn.WriteMessage(websocket.TextMessage, buf); err != nil {
			return err
		}
	}
}
