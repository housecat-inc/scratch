package appserver

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os/exec"
	"strconv"
	"sync"

	"github.com/cockroachdb/errors"
)

const maxLine = 16 << 20

type Client struct {
	cmd     *exec.Cmd
	done    chan struct{}
	err     error
	mu      sync.Mutex
	nextID  int64
	nextSub int
	pending map[int64]chan message
	stdin   io.WriteCloser
	subs    map[int]func(Notification)
	wmu     sync.Mutex
}

func Command() *exec.Cmd {
	return exec.Command("codex", "app-server")
}

func Start(ctx context.Context, cmd *exec.Cmd, info ClientInfo) (*Client, error) {
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stdin pipe")
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, errors.Wrap(err, "stdout pipe")
	}
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "start app-server")
	}
	c := &Client{
		cmd:     cmd,
		done:    make(chan struct{}),
		pending: map[int64]chan message{},
		stdin:   stdin,
		subs:    map[int]func(Notification){},
	}
	go c.readLoop(stdout)

	var res InitializeResponse
	if err := c.Call(ctx, MethodInitialize, InitializeParams{ClientInfo: info}, &res); err != nil {
		_ = c.Close()
		return nil, errors.Wrap(err, "initialize")
	}
	if err := c.Notify(MethodInitialized, nil); err != nil {
		_ = c.Close()
		return nil, errors.Wrap(err, "initialized")
	}
	return c, nil
}

func (c *Client) Call(ctx context.Context, method string, params, result any) error {
	c.mu.Lock()
	if c.err != nil {
		err := c.err
		c.mu.Unlock()
		return err
	}
	c.nextID++
	id := c.nextID
	ch := make(chan message, 1)
	c.pending[id] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, id)
		c.mu.Unlock()
	}()

	raw, err := marshalParams(params)
	if err != nil {
		return err
	}
	if err := c.write(message{ID: json.RawMessage(strconv.FormatInt(id, 10)), Method: method, Params: raw}); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return errors.Wrapf(ctx.Err(), "call %s", method)
	case <-c.done:
		return errors.Wrapf(c.Err(), "call %s", method)
	case m := <-ch:
		if m.Error != nil {
			return errors.Wrapf(m.Error, "call %s", method)
		}
		if result == nil || len(m.Result) == 0 {
			return nil
		}
		return errors.Wrapf(json.Unmarshal(m.Result, result), "decode %s result", method)
	}
}

func (c *Client) Close() error {
	_ = c.stdin.Close()
	if c.cmd.Process != nil {
		_ = c.cmd.Process.Kill()
	}
	<-c.done
	_ = c.cmd.Wait()
	return nil
}

func (c *Client) Done() <-chan struct{} { return c.done }

func (c *Client) Err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.err
}

func (c *Client) LoginCancel(ctx context.Context, loginID string) error {
	return c.Call(ctx, MethodAccountLoginCancel, LoginCancelParams{LoginID: loginID}, nil)
}

func (c *Client) LoginStart(ctx context.Context, p LoginParams) (LoginResponse, error) {
	var res LoginResponse
	err := c.Call(ctx, MethodAccountLoginStart, p, &res)
	return res, err
}

func (c *Client) Notify(method string, params any) error {
	raw, err := marshalParams(params)
	if err != nil {
		return err
	}
	return c.write(message{Method: method, Params: raw})
}

func (c *Client) Subscribe(fn func(Notification)) func() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextSub++
	id := c.nextSub
	c.subs[id] = fn
	return func() {
		c.mu.Lock()
		defer c.mu.Unlock()
		delete(c.subs, id)
	}
}

func (c *Client) dispatch(m message) {
	switch {
	case m.Method != "" && len(m.ID) > 0:
		_ = c.write(message{ID: m.ID, Error: &RPCError{Code: -32601, Message: "unsupported by client: " + m.Method}})
	case m.Method != "":
		c.mu.Lock()
		subs := make([]func(Notification), 0, len(c.subs))
		for _, fn := range c.subs {
			subs = append(subs, fn)
		}
		c.mu.Unlock()
		n := Notification{Method: m.Method, Params: m.Params}
		for _, fn := range subs {
			fn(n)
		}
	case len(m.ID) > 0:
		id, err := strconv.ParseInt(string(m.ID), 10, 64)
		if err != nil {
			return
		}
		c.mu.Lock()
		ch := c.pending[id]
		c.mu.Unlock()
		if ch != nil {
			ch <- m
		}
	}
}

func (c *Client) readLoop(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLine)
	for sc.Scan() {
		var m message
		if err := json.Unmarshal(sc.Bytes(), &m); err != nil {
			continue
		}
		c.dispatch(m)
	}
	err := sc.Err()
	if err == nil {
		err = errors.New("app-server exited")
	}
	c.mu.Lock()
	c.err = err
	c.mu.Unlock()
	close(c.done)
}

func (c *Client) write(m message) error {
	b, err := json.Marshal(m)
	if err != nil {
		return errors.Wrap(err, "marshal message")
	}
	c.wmu.Lock()
	defer c.wmu.Unlock()
	_, err = c.stdin.Write(append(b, '\n'))
	return errors.Wrap(err, "write message")
}

func marshalParams(params any) (json.RawMessage, error) {
	if params == nil {
		return nil, nil
	}
	b, err := json.Marshal(params)
	return b, errors.Wrap(err, "marshal params")
}
