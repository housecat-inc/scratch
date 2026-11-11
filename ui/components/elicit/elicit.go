package elicit

import (
	"encoding/json"

	"github.com/cockroachdb/errors"
)

type FormData struct {
	Checkboxes []CheckboxData
	// Errors is what is wrong with a refused save, by the name of the input
	// it came from: setting.<key> for a checkbox, settings_json for the JSON
	// editor.
	Errors       map[string]string
	HasJSON      bool
	JSONLabel    string
	SettingsJSON string
}

func (data FormData) Configurable() bool {
	return len(data.Checkboxes) > 0 || data.HasJSON
}

type CheckboxData struct {
	Checked bool
	Key     string
	Label   string
}

type Control struct {
	Key   string
	Label string
	Type  string
}

func New(definition any, saved json.RawMessage, controls ...Control) (FormData, error) {
	values := map[string]any{}
	if definition != nil {
		raw, err := json.Marshal(definition)
		if err != nil {
			return FormData{}, errors.Wrap(err, "marshal settings definition")
		}
		if err := json.Unmarshal(raw, &values); err != nil {
			return FormData{}, errors.Wrap(err, "decode settings definition")
		}
	}
	if len(saved) > 0 {
		var overrides map[string]any
		if err := json.Unmarshal(saved, &overrides); err != nil {
			return FormData{}, errors.Wrap(err, "decode saved settings")
		}
		for key, value := range overrides {
			values[key] = value
		}
	}
	out := FormData{JSONLabel: "Settings JSON"}
	for _, control := range controls {
		switch control.Type {
		case "checkbox":
			if checked, ok := values[control.Key].(bool); ok {
				out.Checkboxes = append(out.Checkboxes, CheckboxData{Checked: checked, Key: control.Key, Label: control.Label})
				delete(values, control.Key)
			}
		case "json":
			out.HasJSON = true
			if control.Label != "" {
				out.JSONLabel = control.Label
			}
		}
	}
	raw, err := json.MarshalIndent(values, "", "  ")
	if err != nil {
		return FormData{}, errors.Wrap(err, "encode settings JSON")
	}
	out.SettingsJSON = string(raw)
	return out, nil
}
