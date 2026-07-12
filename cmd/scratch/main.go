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
	"github.com/housecat-inc/scratch/pkg/api"
	"github.com/housecat-inc/scratch/pkg/chat"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/flow"
	"github.com/housecat-inc/scratch/pkg/inbox"
	"github.com/housecat-inc/scratch/pkg/server/code"
	"github.com/housecat-inc/scratch/pkg/server/files"
	"github.com/housecat-inc/scratch/pkg/server/sessions"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/pkg/workflow"
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
	var agentName string
	var port int
	cmd := &cobra.Command{
		Use:     "scratch",
		Short:   "Agent inbox and developer tools",
		Version: fmt.Sprintf("%s (commit %s, built %s)", version, commit, date),
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			slog.SetDefault(logger)

			sessionsSrv, err := sessions.NewServer(sessions.DefaultDeps())
			if err != nil {
				return errors.Wrap(err, "new sessions server")
			}
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

			agent, err := chat.ResolveAgent(agentName)
			if err != nil {
				return err
			}
			chatSvc := chat.NewService(store, agent, logger)
			for _, available := range chat.AvailableAgents() {
				chatSvc.RegisterAgent(chat.AgentName(available), available)
			}
			defer chatSvc.Close()

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
			todoSvc := todo.NewService(store)
			chatSrv := chat.NewServer(chatSvc, logger)
			inboxSrv := inbox.NewServer(todoSvc, chatSvc, logger)

			addr := fmt.Sprintf(":%d", port)
			srv := fuego.NewServer(
				fuego.WithAddr(addr),
				fuego.WithEngineOptions(fuego.WithOpenAPIConfig(fuego.OpenAPIConfig{
					DisableLocalSave: true,
					Info: &openapi3.Info{
						Description: "REST API for scratch workflows and tasks.",
						Title:       "Scratch API",
						Version:     "1.0.0",
					},
				})),
			)
			api.Register(srv, workflows)
			todo.Register(srv, todoSvc)
			sessionsSrv.Register(srv.Mux, false)
			srv.Mux.HandleFunc("/chat", http.NotFound)
			srv.Mux.Handle("/chat/", chatSrv.Handler())
			srv.Mux.Handle("/code/", http.StripPrefix("/code", codeSrv.Handler()))
			srv.Mux.Handle("/files/", http.StripPrefix("/files", filesSrv.Handler()))
			srv.Mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			srv.Mux.Handle("/", inboxSrv.Handler())

			slog.Info("listening", "addr", addr, "agent", agent.Author())
			return srv.Run()
		},
	}
	cmd.Flags().StringVarP(&agentName, "agent", "a", "auto", "chat agent (auto, claude, codex, echo)")
	cmd.Flags().IntVarP(&port, "port", "p", 8888, "HTTP listen port")
	return cmd
}
