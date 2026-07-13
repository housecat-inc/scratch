package flow

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/dbos-inc/dbos-transact-golang/dbos"
	"github.com/housecat-inc/scratch/pkg/elicit"
)

const greetTimeout = 24 * time.Hour

type GreetInput struct{}

type Name struct {
	Name string `json:"name" jsonschema:"What should I call you?"`
}

func (e *Engine) greet(ctx dbos.DBOSContext, _ GreetInput) (string, error) {
	name, action, err := elicit.Form(ctx, "name", "What should I call you?", Name{}, greetTimeout)
	if err != nil {
		return "", errors.Wrap(err, "elicit name")
	}
	if action != elicit.ActionAccept {
		return "", nil
	}
	greeting, err := dbos.RunAsStep(ctx, func(stepCtx context.Context) (string, error) {
		return e.greeter.Greet(stepCtx, strings.TrimSpace(name.Name))
	}, dbos.WithStepName("greeting"))
	if err != nil {
		return "", errors.Wrap(err, "generate greeting")
	}
	return greeting, nil
}

type Greeter interface {
	Greet(ctx context.Context, name string) (string, error)
}

type GreeterFunc func(ctx context.Context, name string) (string, error)

func (f GreeterFunc) Greet(ctx context.Context, name string) (string, error) { return f(ctx, name) }

func DefaultGreeter() Greeter {
	if _, err := exec.LookPath("claude"); err == nil {
		return ClaudeGreeter{}
	}
	return HeuristicGreeter()
}

func HeuristicGreeter() Greeter {
	return GreeterFunc(func(_ context.Context, name string) (string, error) {
		return heuristicGreeting(name), nil
	})
}

type ClaudeGreeter struct {
	Timeout time.Duration
}

func (g ClaudeGreeter) Greet(ctx context.Context, name string) (string, error) {
	timeout := g.Timeout
	if timeout == 0 {
		timeout = 60 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	who := name
	if who == "" {
		who = "a mysterious stranger"
	}
	cmd := exec.CommandContext(ctx, "claude",
		"-p", fmt.Sprintf("Write a single short, warm, playful one-line greeting for %s. Reply with only the greeting, no quotes.", who),
		"--model", "haiku",
		"--output-format", "json",
	)
	out, err := cmd.Output()
	if err != nil {
		return heuristicGreeting(name), nil
	}
	var result struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(out, &result); err != nil || result.IsError {
		return heuristicGreeting(name), nil
	}
	greeting := strings.TrimSpace(result.Result)
	if greeting == "" {
		return heuristicGreeting(name), nil
	}
	return greeting, nil
}

var greetingTemplates = []string{
	"Hey there, %s — great to see you!",
	"Well hello, %s! The day just got brighter.",
	"Greetings, %s! Ready to make something great?",
	"Ahoy, %s! Welcome aboard.",
	"%s! What a delight to have you here.",
}

func heuristicGreeting(name string) string {
	if name == "" {
		name = "friend"
	}
	tmpl := greetingTemplates[len(name)%len(greetingTemplates)]
	return fmt.Sprintf(tmpl, name)
}
