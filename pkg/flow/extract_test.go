package flow

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseContactJSON(t *testing.T) {
	tests := []struct {
		in      string
		name    string
		want    Contact
		wantErr bool
	}{
		{
			in:   `{"company":"Acme Corp","email":"","name":"Jane Doe","notes":""}`,
			name: "clean object",
			want: Contact{Company: "Acme Corp", Name: "Jane Doe"},
		},
		{
			in:   "```json\n{\"company\":\"\",\"email\":\"bob@x.com\",\"name\":\"Bob\",\"notes\":\"met at conf\"}\n```",
			name: "fenced object",
			want: Contact{Email: "bob@x.com", Name: "Bob", Notes: "met at conf"},
		},
		{
			in:   `Here you go: {"name":"Jane"} hope that helps`,
			name: "prose wrapped",
			want: Contact{Name: "Jane"},
		},
		{
			in:      "no json here",
			name:    "missing object",
			wantErr: true,
		},
		{
			in:      `{"name": not-json}`,
			name:    "invalid json",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			got, err := parseContactJSON(tc.in)
			if tc.wantErr {
				a.Error(err)
				return
			}
			a.NoError(err)
			a.Equal(tc.want, got)
		})
	}
}
