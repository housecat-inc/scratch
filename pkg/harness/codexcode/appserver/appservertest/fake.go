package appservertest

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sync"
	"time"
)

const (
	DeviceCode   = "32LO-RHYUX"
	DeviceURL    = "https://auth.openai.com/codex/device"
	EnvComplete  = "FAKE_APPSERVER_DEVICE_COMPLETE"
	EnvEnabled   = "FAKE_APPSERVER"
	GoodCode     = "good"
	LoginID      = "login-1"
	State        = "state-1"
	pingMethod   = "test/ping"
	answerMethod = "test/serverRequestAnswered"
)

func Command(env ...string) *exec.Cmd {
	cmd := exec.Command(os.Args[0], "-test.run=^$")
	cmd.Env = append(os.Environ(), append([]string{EnvEnabled + "=1"}, env...)...)
	return cmd
}

func Enabled() bool { return os.Getenv(EnvEnabled) == "1" }

func Main() {
	f := &fake{out: bufio.NewWriter(os.Stdout)}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 0, 64<<10), 16<<20)
	for sc.Scan() {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		f.handle(m)
	}
	os.Exit(0)
}

type fake struct {
	mu  sync.Mutex
	out *bufio.Writer
}

func (f *fake) handle(m map[string]json.RawMessage) {
	var method string
	_ = json.Unmarshal(m["method"], &method)
	id, hasID := m["id"]
	if method == "" && hasID {
		if string(id) == `"srv-1"` && m["error"] != nil {
			f.send(map[string]any{"method": answerMethod, "params": map[string]any{}})
		}
		return
	}
	if !hasID {
		return
	}
	switch method {
	case "initialize":
		f.reply(id, map[string]any{"codexHome": "/tmp/fake", "userAgent": "fake/0"})
	case "account/login/start":
		var p struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(m["params"], &p)
		f.loginStart(id, p.Type)
	case "account/login/cancel":
		f.reply(id, map[string]any{})
		f.completed(false, "cancelled")
	case pingMethod:
		f.reply(id, map[string]any{"pong": true})
		f.send(map[string]any{"id": "srv-1", "method": "item/tool/requestUserInput", "params": map[string]any{}})
	default:
		f.send(map[string]any{"id": json.RawMessage(id), "error": map[string]any{"code": -32601, "message": "method not found"}})
	}
}

func (f *fake) loginStart(id json.RawMessage, typ string) {
	switch typ {
	case "chatgpt":
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			f.send(map[string]any{"id": id, "error": map[string]any{"code": -32000, "message": err.Error()}})
			return
		}
		go func() { _ = http.Serve(ln, http.HandlerFunc(f.callback)) }()
		redirect := fmt.Sprintf("http://localhost:%d/auth/callback", ln.Addr().(*net.TCPAddr).Port)
		auth := "https://auth.openai.com/oauth/authorize?" + url.Values{"redirect_uri": {redirect}, "state": {State}}.Encode()
		f.reply(id, map[string]any{"authUrl": auth, "loginId": LoginID, "type": "chatgpt"})
	case "chatgptDeviceCode":
		f.reply(id, map[string]any{"loginId": LoginID, "type": "chatgptDeviceCode", "userCode": DeviceCode, "verificationUrl": DeviceURL})
		if os.Getenv(EnvComplete) == "1" {
			go func() {
				time.Sleep(50 * time.Millisecond)
				f.completed(true, "")
			}()
		}
	default:
		f.send(map[string]any{"id": id, "error": map[string]any{"code": -32602, "message": "bad login type"}})
	}
}

func (f *fake) callback(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	switch {
	case r.URL.Path != "/auth/callback":
		http.NotFound(w, r)
		return
	case q.Get("state") != State:
		f.completed(false, "state mismatch")
	case q.Get("code") == GoodCode:
		f.completed(true, "")
	default:
		f.completed(false, "token exchange failed")
	}
	_, _ = w.Write([]byte("<html>ok</html>"))
}

func (f *fake) completed(success bool, errMsg string) {
	params := map[string]any{"loginId": LoginID, "success": success}
	if errMsg != "" {
		params["error"] = errMsg
	}
	f.send(map[string]any{"method": "account/login/completed", "params": params})
}

func (f *fake) reply(id json.RawMessage, result any) {
	f.send(map[string]any{"id": id, "result": result})
}

func (f *fake) send(v any) {
	b, _ := json.Marshal(v)
	f.mu.Lock()
	defer f.mu.Unlock()
	_, _ = f.out.Write(append(b, '\n'))
	_ = f.out.Flush()
}
