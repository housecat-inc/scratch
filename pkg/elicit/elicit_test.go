package elicit

import (
	"encoding/json"
	"testing"

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

func TestElicit(t *testing.T) {
	t.Run("Coerce", func(t *testing.T) {
		tests := []struct {
			in      map[string]string
			invalid bool
			name    string
			want    map[string]any
		}{
			{
				in:   map[string]string{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": "3", "urgent": "on"},
				name: "converts a full form to typed values",
				want: map[string]any{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": int64(3), "urgent": true},
			},
			{
				in:   map[string]string{"email": "j@x.com", "name": "Jane"},
				name: "treats an unchecked checkbox as false",
				want: map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
			},
			{in: map[string]string{"name": "Jane"}, invalid: true, name: "rejects a missing required field"},
			{in: map[string]string{"email": "j@x.com", "name": "Jane", "retries": "lots"}, invalid: true, name: "rejects a non-integer for an integer field"},
			{
				in:   map[string]string{"email": "j@x.com", "name": "Jane", "sneaky": "x"},
				name: "drops fields not in the schema",
				want: map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
			},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				schema, err := SchemaFor(contact{})
				r.NoError(err)

				got, err := Coerce(schema, tt.in)
				if tt.invalid {
					a.True(IsInvalid(err))
					return
				}
				r.NoError(err)
				a.Equal(tt.want, got)
			})
		}
	})

	t.Run("SchemaFor", func(t *testing.T) {
		t.Run("describes fields with json types in declaration order", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			schema, err := SchemaFor(contact{Email: "jane@example.com", Name: "Jane"})
			r.NoError(err)

			a.Equal("object", schema.Type)
			a.Equal([]string{"company", "email", "name", "retries", "urgent"}, schema.PropertyOrder)
			a.Equal("Company name", schema.Properties["company"].Description)
			a.Equal("string", schema.Properties["email"].Type)
			a.Equal("integer", schema.Properties["retries"].Type)
			a.Equal("boolean", schema.Properties["urgent"].Type)
		})
		t.Run("requires fields without omitempty", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			schema, err := SchemaFor(contact{Email: "jane@example.com", Name: "Jane"})
			r.NoError(err)

			a.ElementsMatch([]string{"email", "name"}, schema.Required)
		})
		t.Run("defaults fields to their non-zero values", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			schema, err := SchemaFor(contact{Email: "jane@example.com", Name: "Jane"})
			r.NoError(err)

			a.Equal(json.RawMessage(`"jane@example.com"`), schema.Properties["email"].Default)
			a.Equal(json.RawMessage(`"Jane"`), schema.Properties["name"].Default)
			a.Nil(schema.Properties["company"].Default)
			a.Nil(schema.Properties["urgent"].Default)
		})
		t.Run("survives a json round trip inside a prompt", func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			schema, err := SchemaFor(contact{Email: "jane@example.com", Name: "Jane"})
			r.NoError(err)

			data, err := json.Marshal(Prompt{Order: schema.PropertyOrder, RequestedSchema: schema})
			r.NoError(err)
			var back Prompt
			r.NoError(json.Unmarshal(data, &back))
			a.Equal("string", back.RequestedSchema.Properties["email"].Type)
			a.Equal(schema.PropertyOrder, back.Order)
		})
	})
}
