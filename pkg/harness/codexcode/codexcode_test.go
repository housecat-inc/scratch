package codexcode

import (
	"fmt"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexcode(t *testing.T) {
	t.Run("version", func(t *testing.T) {
		tests := []struct {
			exit    int
			name    string
			stdout  string
			want    string
			wantErr bool
		}{
			{name: "strips the codex-cli prefix", stdout: "codex-cli 0.5.2\n", want: "0.5.2"},
			{name: "returns a bare version", stdout: "0.5.2\n", want: "0.5.2"},
			{name: "returns empty for empty output", stdout: "", want: ""},
			{name: "errors when the command fails", exit: 1, stdout: "boom", wantErr: true},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				a := assert.New(t)
				r := require.New(t)

				cmd := exec.Command("sh", "-c", fmt.Sprintf("printf %q; exit %d", tt.stdout, tt.exit))
				got, err := version(cmd)
				if tt.wantErr {
					a.Error(err)
					return
				}
				r.NoError(err)
				a.Equal(tt.want, got)
			})
		}
	})
}
