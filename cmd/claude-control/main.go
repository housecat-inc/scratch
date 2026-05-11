package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/diffview"
	"github.com/housecat-inc/scratch/pkg/harness/claudecode/remote"
	"github.com/spf13/cobra"
)

var (
	commit  = "none"
	date    = "unknown"
	version = "dev"
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
		Use:     "claude-control",
		Short:   "Web UI for installing, unleashing, and connecting claude",
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
			s, err := remote.NewServer(remote.DefaultDeps())
			if err != nil {
				return errors.Wrap(err, "new server")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return errors.Wrap(err, "user home dir")
			}
			conn, err := db.Open(filepath.Join(home, ".config", "scratch", "diff.db"))
			if err != nil {
				return errors.Wrap(err, "open db")
			}
			defer conn.Close()
			deps := diffview.DefaultDeps(home)
			deps.Comments = diffview.NewSQLiteCommentStore(conn)
			dv, err := diffview.NewServer(deps)
			if err != nil {
				return errors.Wrap(err, "new diffview server")
			}

			mux := http.NewServeMux()
			mux.Handle("/diff/", http.StripPrefix("/diff", dv.Handler()))
			mux.Handle("/", s.Handler())

			addr := fmt.Sprintf(":%d", port)
			slog.Info("listening", "addr", addr)
			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8888, "HTTP listen port")
	return cmd
}
