package main

import (
	"fmt"
	"net/http"
	"os"

	"github.com/cockroachdb/errors"
	"github.com/nzoschke/medan/pkg/harness/claudecode/remote"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var port int
	cmd := &cobra.Command{
		Use:   "claude-remote",
		Short: "Web UI for installing, configuring, and logging into claude",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := remote.NewServer(remote.DefaultDeps())
			if err != nil {
				return errors.Wrap(err, "new server")
			}
			addr := fmt.Sprintf(":%d", port)
			fmt.Printf("listening on http://localhost%s\n", addr)
			return http.ListenAndServe(addr, s.Handler())
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 9999, "HTTP listen port")
	return cmd
}
