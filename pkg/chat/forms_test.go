package chat

import (
	"encoding/json"
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormProps(t *testing.T) {
	t.Run("unescapes html entities in field defaults", func(t *testing.T) {
		a := assert.New(t)
		r := require.New(t)

		form := FormProps("/submit", elicit.Prompt{
			ElicitationID: "pr/review",
			Message:       "Review",
			Order:         []string{"title"},
			RequestedSchema: &jsonschema.Schema{
				Properties: map[string]*jsonschema.Schema{
					"title": {
						Default: json.RawMessage(`"Add edit &amp; fork actions"`),
						Type:    "string",
					},
				},
				Type: "object",
			},
		})

		r.Len(form.Fields, 1)
		a.Equal("Add edit & fork actions", form.Fields[0].Value)
	})
}
