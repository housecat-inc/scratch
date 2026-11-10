package codexcode

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/harness/codexcode/appserver"
)

var (
	callbackTimeout   = 30 * time.Second
	clientInfo        = appserver.ClientInfo{Name: "scratch", Title: "scratch", Version: "dev"}
	loginStartTimeout = 30 * time.Second
)

type Login struct {
	client   *appserver.Client
	done     chan struct{}
	err      error
	finished bool
	mu       sync.Mutex
	res      appserver.LoginResponse
	unsub    func()
}

func StartBrowserLogin() (*Login, error) {
	return StartLoginCmd(appserver.Command(), appserver.LoginTypeChatGPT)
}

func StartDeviceLogin() (*Login, error) {
	return StartLoginCmd(appserver.Command(), appserver.LoginTypeDeviceCode)
}

func StartLoginCmd(cmd *exec.Cmd, loginType string) (*Login, error) {
	ctx, cancel := context.WithTimeout(context.Background(), loginStartTimeout)
	defer cancel()

	c, err := appserver.Start(ctx, cmd, clientInfo)
	if err != nil {
		return nil, errors.Wrap(err, "start codex app-server")
	}
	l := &Login{client: c, done: make(chan struct{})}
	l.unsub = c.Subscribe(l.onNotification)
	go func() {
		<-c.Done()
		l.finish(errors.New("codex app-server exited before login completed"))
	}()

	res, err := c.LoginStart(ctx, appserver.LoginParams{Type: loginType})
	if err != nil {
		_ = l.Close()
		return nil, errors.Wrap(err, "start codex login")
	}
	l.mu.Lock()
	l.res = res
	l.mu.Unlock()
	return l, nil
}

func (l *Login) Close() error {
	l.mu.Lock()
	finished, id := l.finished, l.res.LoginID
	l.mu.Unlock()
	if !finished && id != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = l.client.LoginCancel(ctx, id)
		cancel()
	}
	l.unsub()
	return l.client.Close()
}

func (l *Login) Code() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.res.UserCode
}

func (l *Login) Done() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.finished
}

func (l *Login) Err() error {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.err
}

func (l *Login) SubmitCallback(raw string) error {
	target, err := l.callbackTarget(raw)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), callbackTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return errors.Wrap(err, "build callback request")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "forward callback to codex")
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()

	select {
	case <-l.done:
		return l.Err()
	case <-ctx.Done():
		return errors.New("codex did not finish sign in")
	}
}

func (l *Login) URL() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.res.AuthURL != "" {
		return l.res.AuthURL
	}
	return l.res.VerificationURL
}

func (l *Login) callbackTarget(raw string) (string, error) {
	l.mu.Lock()
	authURL := l.res.AuthURL
	l.mu.Unlock()
	if authURL == "" {
		return "", errors.New("not a browser sign in")
	}

	au, err := url.Parse(authURL)
	if err != nil {
		return "", errors.Wrap(err, "parse auth url")
	}
	redirect, err := url.Parse(au.Query().Get("redirect_uri"))
	if err != nil || redirect.Port() == "" {
		return "", errors.New("auth url has no local redirect")
	}

	pasted, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", errors.New("paste the full URL from your browser's address bar")
	}
	q := pasted.Query()
	if q.Get("state") == "" || (q.Get("code") == "" && q.Get("error") == "") {
		return "", errors.New("paste the full URL from your browser's address bar")
	}
	if pasted.Path != redirect.Path {
		return "", errors.Errorf("expected a %s URL", redirect.Path)
	}

	return (&url.URL{
		Host:     net.JoinHostPort("127.0.0.1", redirect.Port()),
		Path:     redirect.Path,
		RawQuery: pasted.RawQuery,
		Scheme:   "http",
	}).String(), nil
}

func (l *Login) finish(err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.finished {
		return
	}
	l.finished = true
	l.err = err
	close(l.done)
}

func (l *Login) onNotification(n appserver.Notification) {
	if n.Method != appserver.MethodAccountLoginCompleted {
		return
	}
	var lc appserver.LoginCompleted
	if err := json.Unmarshal(n.Params, &lc); err != nil {
		return
	}
	if lc.Success {
		l.finish(nil)
		return
	}
	msg := "codex sign in failed"
	if lc.Error != nil && *lc.Error != "" {
		msg = *lc.Error
	}
	l.finish(errors.New(msg))
}
