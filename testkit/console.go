package testkit

import (
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

type ConsoleMessage struct {
	Text string
	Type string
}

type ConsoleRecorder struct {
	messages []ConsoleMessage
	mu       sync.Mutex
}

func NewConsoleRecorder(t *testing.T, artifacts *Artifacts, page *rod.Page) *ConsoleRecorder {
	t.Helper()

	recorder := &ConsoleRecorder{}

	go page.EachEvent(func(e *proto.RuntimeConsoleAPICalled) {
		recorder.add(string(e.Type), consoleText(page, e.Args))
	}, func(e *proto.RuntimeExceptionThrown) {
		recorder.add("exception", exceptionText(e.ExceptionDetails))
	})()

	t.Cleanup(func() {
		messages := recorder.All()
		if len(messages) == 0 {
			return
		}

		lines := make([]string, 0, len(messages))
		for _, message := range messages {
			bytes, err := json.Marshal(message)
			if err != nil {
				t.Error(err)
				return
			}
			lines = append(lines, string(bytes))
		}
		out := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(artifacts.ActualPath("console.jsonl"), []byte(out), 0o644); err != nil {
			t.Error(err)
		}
	})

	return recorder
}

func (c *ConsoleRecorder) All() []ConsoleMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ConsoleMessage{}, c.messages...)
}

func (c *ConsoleRecorder) Errors() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := []string{}
	for _, message := range c.messages {
		if message.Type == "error" || message.Type == "exception" {
			out = append(out, message.Text)
		}
	}
	return out
}

func (c *ConsoleRecorder) add(kind string, text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, ConsoleMessage{Text: text, Type: kind})
}

func consoleText(page *rod.Page, args []*proto.RuntimeRemoteObject) (out string) {
	defer func() {
		if r := recover(); r != nil {
			parts := make([]string, 0, len(args))
			for _, arg := range args {
				parts = append(parts, arg.Description)
			}
			out = strings.Join(parts, " ")
		}
	}()
	return page.MustObjectsToJSON(args).String()
}

func exceptionText(details *proto.RuntimeExceptionDetails) string {
	if details.Exception != nil && details.Exception.Description != "" {
		return details.Exception.Description
	}
	return details.Text
}
