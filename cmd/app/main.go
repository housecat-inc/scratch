package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/todo"
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
		Use:   "app",
		Short: "TodoMVC CRUD app",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			slog.SetDefault(logger)

			home, err := os.UserHomeDir()
			if err != nil {
				return errors.Wrap(err, "user home dir")
			}
			dbPath := filepath.Join(home, ".config", "scratch", "scratch.db")
			store, err := db.New(dbPath)
			if err != nil {
				return errors.Wrap(err, "open db")
			}
			defer store.Close()

			svc := todo.NewService(store)
			addr := fmt.Sprintf(":%d", port)
			srv := fuego.NewServer(
				fuego.WithAddr(addr),
				fuego.WithEngineOptions(fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
					DisableLocalSave: true,
					Info: &openapi3.Info{
						Description: "REST API for the scratch TodoMVC app: create, list, read, update, and delete tasks.",
						Title:       "Tasks API",
						Version:     "1.0.0",
					},
				})),
			)
			todo.Register(srv, svc)
			srv.Mux.Handle("/", todo.NewServer(svc, logger).Handler())

			slog.Info("listening", "addr", addr)
			return srv.Run()
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8000, "HTTP listen port")
	return cmd
}
