package elicit

import (
	"encoding/json"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/google/jsonschema-go/jsonschema"
)

const (
	ActionAccept  = "accept"
	ActionCancel  = "cancel"
	ActionDecline = "decline"
)

type Prompt struct {
	ElicitationID   string             `json:"elicitationId"`
	Message         string             `json:"message"`
	Order           []string           `json:"order"`
	RequestedSchema *jsonschema.Schema `json:"requestedSchema"`
	Topic           string             `json:"topic"`
	WorkflowID      string             `json:"workflowId"`
}

type Reply struct {
	Action  string
	Content string
}

var ErrInvalid = errors.New("invalid form input")

func IsInvalid(err error) bool { return errors.Is(err, ErrInvalid) }

func SchemaFor[T any](defaults T) (*jsonschema.Schema, error) {
	schema, err := jsonschema.For[T](nil)
	if err != nil {
		return nil, errors.Wrap(err, "derive schema")
	}
	raw, err := json.Marshal(defaults)
	if err != nil {
		return nil, errors.Wrap(err, "marshal defaults")
	}
	values := map[string]json.RawMessage{}
	if err := json.Unmarshal(raw, &values); err != nil {
		return nil, errors.Wrap(err, "unmarshal defaults")
	}
	for name, value := range values {
		prop, ok := schema.Properties[name]
		if !ok || isZeroJSON(value) {
			continue
		}
		prop.Default = value
	}
	return schema, nil
}

func Coerce(schema *jsonschema.Schema, values map[string]string) (map[string]any, error) {
	out := map[string]any{}
	for name, prop := range schema.Properties {
		raw, ok := values[name]
		switch prop.Type {
		case "boolean":
			out[name] = ok && (raw == "on" || raw == "true")
		case "integer":
			if raw == "" {
				continue
			}
			n, err := strconv.ParseInt(raw, 10, 64)
			if err != nil {
				return nil, errors.Wrapf(ErrInvalid, "%s must be an integer", name)
			}
			out[name] = n
		case "number":
			if raw == "" {
				continue
			}
			n, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, errors.Wrapf(ErrInvalid, "%s must be a number", name)
			}
			out[name] = n
		default:
			if !ok {
				continue
			}
			out[name] = raw
		}
	}
	resolved, err := schema.Resolve(nil)
	if err != nil {
		return nil, errors.Wrap(err, "resolve schema")
	}
	if err := resolved.Validate(out); err != nil {
		return nil, errors.Wrapf(ErrInvalid, "%s", err.Error())
	}
	return out, nil
}

func isZeroJSON(v json.RawMessage) bool {
	switch string(v) {
	case `""`, "0", "false", "null":
		return true
	}
	return false
}
