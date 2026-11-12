package chat

import (
	"encoding/json"
	"html"
	"strings"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/ui"
)

func FormProps(action string, prompt elicit.Prompt) ui.ChatFormProps {
	form := ui.ChatFormProps{
		AcceptLabel:   prompt.AcceptLabel,
		Action:        action,
		DeclineLabel:  prompt.DeclineLabel,
		ElicitationID: prompt.ElicitationID,
		Message:       prompt.Message,
	}
	schema := prompt.RequestedSchema
	if schema == nil {
		return form
	}
	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}
	for _, name := range prompt.Order {
		prop, ok := schema.Properties[name]
		if !ok {
			continue
		}
		form.Fields = append(form.Fields, toFieldProps(name, prop, required[name]))
	}
	return form
}

func toFieldProps(name string, prop *jsonschema.Schema, required bool) ui.ChatFormFieldProps {
	field := ui.ChatFormFieldProps{
		Label:    prop.Title,
		Name:     name,
		Required: required,
		Type:     "text",
	}
	if field.Label == "" {
		field.Label = name
	}
	switch prop.Type {
	case "boolean":
		field.Type = "checkbox"
	case "integer", "number":
		field.Type = "number"
	}
	if prop.Format == "email" {
		field.Type = "email"
	}
	switch name {
	case "body", "description", "notes":
		field.Type = "textarea"
	}
	if name == "contact" || strings.HasSuffix(name, "_contact") {
		field.Type = "contact"
	}
	for _, option := range prop.Enum {
		if s, ok := option.(string); ok {
			field.Options = append(field.Options, s)
		}
	}
	if len(field.Options) > 0 {
		field.Type = "select"
	}
	if len(prop.Default) > 0 {
		var s string
		if err := json.Unmarshal(prop.Default, &s); err == nil {
			field.Value = html.UnescapeString(s)
		} else {
			field.Value = html.UnescapeString(string(prop.Default))
		}
	}
	return field
}
