package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/server/code"
	"github.com/housecat-inc/scratch/pkg/server/files"
	"github.com/housecat-inc/scratch/pkg/server/sessions"
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
			sessionsSrv, err := sessions.NewServer(sessions.DefaultDeps())
			if err != nil {
				return errors.Wrap(err, "new sessions server")
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return errors.Wrap(err, "user home dir")
			}
			store, err := db.New(filepath.Join(home, ".config", "scratch", "scratch.db"))
			if err != nil {
				return errors.Wrap(err, "open db")
			}
			defer store.Close()
			codeDeps := code.DefaultDeps(home)
			codeDeps.Comments = store
			codeSrv, err := code.NewServer(codeDeps)
			if err != nil {
				return errors.Wrap(err, "new code server")
			}
			filesSrv, err := files.NewServer(files.DefaultDeps(home))
			if err != nil {
				return errors.Wrap(err, "new files server")
			}

			mux := http.NewServeMux()
			mux.Handle("/code/", http.StripPrefix("/code", codeSrv.Handler()))
			mux.Handle("/files/", http.StripPrefix("/files", filesSrv.Handler()))
			mux.Handle("/", sessionsSrv.Handler())

			addr := fmt.Sprintf(":%d", port)
			slog.Info("listening", "addr", addr)
			return http.ListenAndServe(addr, mux)
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8888, "HTTP listen port")
	return cmd
}
