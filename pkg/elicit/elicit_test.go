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

	tests := []struct {
		name    string
		values  map[string]string
		want    map[string]any
		wantErr bool
	}{
		{
			name:   "accepts full form",
			values: map[string]string{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": "3", "urgent": "on"},
			want:   map[string]any{"company": "ACME", "email": "j@x.com", "name": "Jane", "retries": int64(3), "urgent": true},
		},
		{
			name:   "unchecked checkbox is false",
			values: map[string]string{"email": "j@x.com", "name": "Jane"},
			want:   map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
		},
		{
			name:    "missing required field",
			values:  map[string]string{"name": "Jane"},
			wantErr: true,
		},
		{
			name:    "non-integer retries",
			values:  map[string]string{"email": "j@x.com", "name": "Jane", "retries": "lots"},
			wantErr: true,
		},
		{
			name:   "ignores unknown fields",
			values: map[string]string{"email": "j@x.com", "name": "Jane", "sneaky": "x"},
			want:   map[string]any{"email": "j@x.com", "name": "Jane", "urgent": false},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := Coerce(schema, tc.values)
			if tc.wantErr {
				a.True(IsInvalid(err))
				return
			}
			a.NoError(err)
			a.Equal(tc.want, got)
		})
	}
}
