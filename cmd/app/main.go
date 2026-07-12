package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/spf13/cobra"
)

func main() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCmd() *cobra.Command {
	var agentName string
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

			agent, err := chat.ResolveAgent(agentName)
			if err != nil {
				return err
			}
			chatSvc := chat.NewService(store, agent, logger)
			defer chatSvc.Close()

			workflows, err := workflow.New(dbPath)
			if err != nil {
				return errors.Wrap(err, "open workflows")
			}
			defer workflows.Close()
			flows := flow.New(flow.Deps{
				DBOS:    workflows.Ctx(),
				Log:     logger,
				Publish: chatSvc.Publish,
				Store:   store,
				Tasks:   store,
			})
			chatSvc.RegisterAgent("contact", flows.Agent())
			chatSvc.SetResolver(flows)
			if err := workflows.Launch(); err != nil {
				return errors.Wrap(err, "launch workflows")
			}
			if err := chatSvc.Recover(); err != nil {
				return errors.Wrap(err, "recover chat turns")
			}

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
			chatSrv := chat.NewServer(chatSvc, logger)
			srv.Mux.Handle("/chat", chatSrv.Handler())
			srv.Mux.Handle("/chat/", chatSrv.Handler())
			srv.Mux.Handle("/", todo.NewServer(svc, logger).Handler())

			slog.Info("listening", "addr", addr, "agent", agent.Author())
			return srv.Run()
		},
	}
	cmd.Flags().StringVarP(&agentName, "agent", "a", "auto", "chat agent (auto, claude, echo)")
	cmd.Flags().IntVarP(&port, "port", "p", 8000, "HTTP listen port")
	return cmd
}
