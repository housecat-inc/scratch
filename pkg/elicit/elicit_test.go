package elicit

import (
	"encoding/json"
	"testing"

	tk "github.com/housecat-inc/scratch/testkit/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contact struct {
	Company string `json:"company,omitempty" jsonschema:"Company name"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	Retries int64  `json:"retries,omitempty"`
	Urgent  bool   `json:"urgent,omitempty"`
}

func TestSchemaFor(t *testing.T) {
	a := assert.New(t)
	r := require.New(t)

	schema, err := SchemaFor(contact{Email: "jane@example.com", Name: "Jane"})
	r.NoError(err)

	a.Equal("object", schema.Type)
	a.Equal([]string{"company", "email", "name", "retries", "urgent"}, schema.PropertyOrder)
	a.ElementsMatch([]string{"email", "name"}, schema.Required)
	a.Equal("Company name", schema.Properties["company"].Description)
	a.Equal("string", schema.Properties["email"].Type)
	a.Equal("integer", schema.Properties["retries"].Type)
	a.Equal("boolean", schema.Properties["urgent"].Type)

	a.Equal(json.RawMessage(`"jane@example.com"`), schema.Properties["email"].Default)
	a.Equal(json.RawMessage(`"Jane"`), schema.Properties["name"].Default)
	a.Nil(schema.Properties["company"].Default)
	a.Nil(schema.Properties["urgent"].Default)

	data, err := json.Marshal(Prompt{Order: schema.PropertyOrder, RequestedSchema: schema})
	r.NoError(err)
	var back Prompt
	r.NoError(json.Unmarshal(data, &back))
	a.Equal("string", back.RequestedSchema.Properties["email"].Type)
	a.Equal(schema.PropertyOrder, back.Order)
}

func TestCoerce(t *testing.T) {
	schema, err := SchemaFor(contact{})
	require.NoError(t, err)

	invalid := func(t *tk.T, _ map[string]any, err error) { t.A.True(IsInvalid(err)) }

	tk.Run(t, []tk.Test[map[string]string, map[string]any]{
		{
			Name: "accepts full form",
			In:   map[string]string{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": "3", "urgent": "on"},
			Out:  map[string]any{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": int64(3), "urgent": true},
		},
		{
			Name: "unchecked checkbox is false",
			In:   map[string]string{"email": "j@x.com", "name": "Jane"},
			Out:  map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
		},
		{Name: "missing required field", In: map[string]string{"name": "Jane"}, Assert: invalid},
		{Name: "non-integer retries", In: map[string]string{"email": "j@x.com", "name": "Jane", "retries": "lots"}, Assert: invalid},
		{
			Name: "ignores unknown fields",
			In:   map[string]string{"email": "j@x.com", "name": "Jane", "sneaky": "x"},
			Out:  map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
		},
	}, func(values map[string]string) (map[string]any, error) {
		return Coerce(schema, values)
	})
}
