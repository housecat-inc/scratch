package codexcode

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVersion(t *testing.T) {
	tests := []struct {
		exit    int
		name    string
		stdout  string
		want    string
		wantErr bool
	}{
		{name: "codex-cli prefix", stdout: "codex-cli 0.5.2\n", want: "0.5.2"},
		{name: "bare version", stdout: "0.5.2\n", want: "0.5.2"},
		{name: "empty output", stdout: "", want: ""},
		{name: "command error", exit: 1, stdout: "boom", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := assert.New(t)
			r := require.New(t)

			cmd := exec.Command("sh", "-c", fmt.Sprintf("printf %q; exit %d", tc.stdout, tc.exit))
			got, err := version(cmd)
			if tc.wantErr {
				a.Error(err)
				return
			}
			r.NoError(err)
			a.Equal(tc.want, got)
		})
	}
}
