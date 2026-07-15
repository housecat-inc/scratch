package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-fuego/fuego"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
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
	var port int
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Todo list demo app",
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
			workflows, err := workflow.New(dbPath)
			if err != nil {
				return errors.Wrap(err, "open workflows")
			}
			defer workflows.Close()

			workdir, err := os.Getwd()
			if err != nil {
				return errors.Wrap(err, "working dir")
			}
			agent, err := chat.ResolveAgentInDir("auto", workdir)
			if err != nil {
				return err
			}
			chatSvc := chat.NewService(store, agent, logger)
			for _, available := range chat.AvailableAgentsInDir(workdir) {
				chatSvc.RegisterAgent(chat.AgentName(available), available)
			}
			defer chatSvc.Close()

			flows := flow.New(flow.Deps{
				DBOS:    workflows.Ctx(),
				Log:     logger,
				Workdir: workdir,
			})
			chatSvc.SetResolver(flows)
			if err := workflows.Launch(); err != nil {
				return errors.Wrap(err, "launch workflows")
			}
			if err := flows.EnsureSchedules(); err != nil {
				return errors.Wrap(err, "ensure schedules")
			}
			if err := chatSvc.Recover(); err != nil {
				return errors.Wrap(err, "recover chat turns")
			}

			svc := todo.NewService(store)
			chatSrv := chat.NewServer(chatSvc, logger)
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
			srv.Mux.HandleFunc("/chat", http.NotFound)
			srv.Mux.Handle("/chat/", chatSrv.Handler())
			srv.Mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			webSrv := todo.NewWebServerWithChat(svc, chatSvc, logger)
			webSrv.SetContacts(store)
			srv.Mux.Handle("/", webSrv.Handler())

			slog.Info("listening", "addr", addr)
			return srv.Run()
		},
	}
	cmd.Flags().IntVarP(&port, "port", "p", 8000, "HTTP listen port")
	return cmd
}
