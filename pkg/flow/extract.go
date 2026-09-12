package flow

import (
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/cockroachdb/errors"
	"github.com/google/jsonschema-go/jsonschema"
)

const extractPrompt = `Extract contact details from the message below.
Use "" for any field not present in the message — never invent values. Capitalize names properly.

Message: `

type Extractor interface {
	Extract(ctx context.Context, message string) (Contact, error)
}

type ExtractorFunc func(ctx context.Context, message string) (Contact, error)

func (f ExtractorFunc) Extract(ctx context.Context, message string) (Contact, error) {
	return f(ctx, message)
}

func DefaultExtractor() Extractor {
	if _, err := exec.LookPath("claude"); err == nil {
		return ClaudeExtractor{}
	}
	return HeuristicExtractor()
}

func HeuristicExtractor() Extractor {
	return ExtractorFunc(func(_ context.Context, message string) (Contact, error) {
		return draftContact(message), nil
	})
}

type ClaudeExtractor struct {
	Timeout time.Duration
}

func (e ClaudeExtractor) Extract(ctx context.Context, message string) (Contact, error) {
	timeout := e.Timeout
	if timeout == 0 {
		timeout = 90 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	schema, err := contactSchema()
	if err != nil {
		return Contact{}, err
	}
	cmd := exec.CommandContext(ctx, "claude",
		"-p", extractPrompt+message,
		"--model", "haiku",
		"--output-format", "json",
		"--json-schema", schema,
	)
	out, err := cmd.Output()
	if err != nil {
		return Contact{}, errors.Wrap(err, "run claude extract")
	}
	var result struct {
		IsError bool   `json:"is_error"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return Contact{}, errors.Wrap(err, "decode claude output")
	}
	if result.IsError {
		return Contact{}, errors.Newf("claude extract: %s", result.Result)
	}
	var contact Contact
	if err := json.Unmarshal([]byte(result.Result), &contact); err == nil {
		return contact, nil
	}
	return parseContactJSON(result.Result)
}

func contactSchema() (string, error) {
	schema, err := jsonschema.For[Contact](nil)
	if err != nil {
		return "", errors.Wrap(err, "derive contact schema")
	}
	schema.Required = nil
	data, err := json.Marshal(schema)
	if err != nil {
		return "", errors.Wrap(err, "marshal contact schema")
	}
	return string(data), nil
}

func parseContactJSON(s string) (Contact, error) {
	start := strings.Index(s, "{")
	end := strings.LastIndex(s, "}")
	if start < 0 || end <= start {
		return Contact{}, errors.Newf("no JSON object in %q", s)
	}
	var contact Contact
	if err := json.Unmarshal([]byte(s[start:end+1]), &contact); err != nil {
		return Contact{}, errors.Wrap(err, "decode contact")
	}
	return contact, nil
}
