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
	"github.com/housecat-inc/scratch/pkg/repo"
	"github.com/housecat-inc/scratch/pkg/server/code"
	"github.com/housecat-inc/scratch/pkg/server/files"
	"github.com/housecat-inc/scratch/pkg/server/sessions"
	sqlsrv "github.com/housecat-inc/scratch/pkg/server/sql"
	"github.com/housecat-inc/scratch/pkg/todo"
	"github.com/housecat-inc/scratch/pkg/ui"
	"github.com/housecat-inc/scratch/pkg/workflow"
	"github.com/housecat-inc/scratch/pkg/workspace"
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

			ws, err := workspace.Resolve()
			if err != nil {
				return err
			}
			home, workdir := ws.Home, ws.Dir
			var chatSvc *chat.Service
			var todoSvc *todo.Service
			shell := func() ui.ToolShellProps {
				return toolShell(todoSvc, chatSvc)
			}

			sessionsDeps := sessions.DefaultDeps()
			sessionsDeps.Shell = shell
			sessionsSrv, err := sessions.NewServer(sessionsDeps, workdir)
			if err != nil {
				return errors.Wrap(err, "new sessions server")
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

			agent, err := chat.ResolveAgentInDir(agentName, workdir)
			if err != nil {
				return err
			}
			chatSvc = chat.NewService(store, agent, logger)
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

			codeDeps := code.DefaultDeps(home)
			codeDeps.Comments = store
			codeDeps.ListRepos = func() ([]repo.Repo, error) { return repo.ScanIncluding(home, workdir) }
			codeDeps.LookupRepo = func(slug string) (repo.Repo, bool) { return repo.FindIncluding(home, slug, workdir) }
			codeDeps.Shell = shell
			codeSrv, err := code.NewServer(codeDeps)
			if err != nil {
				return errors.Wrap(err, "new code server")
			}
			filesDeps := files.DefaultDeps(workdir)
			filesDeps.Shell = shell
			filesSrv, err := files.NewServer(filesDeps)
			if err != nil {
				return errors.Wrap(err, "new files server")
			}
			sqlDeps := sqlsrv.DefaultDeps(home, store)
			sqlDeps.Shell = shell
			sqlSrv, err := sqlsrv.NewServer(sqlDeps)
			if err != nil {
				return errors.Wrap(err, "new sql server")
			}
			todoSvc = todo.NewService(store)
			chatSrv := chat.NewServer(chatSvc, logger)
			inboxSrv := inbox.NewServer(todoSvc, chatSvc, flows, logger)

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
			srv.Mux.Handle("/sql/", http.StripPrefix("/sql", sqlSrv.Handler()))
			srv.Mux.Handle("/static/", http.StripPrefix("/static/", ui.StaticHandler()))
			srv.Mux.Handle("/", inboxSrv.Handler())

			slog.Info("listening", "addr", addr, "agent", agent.Author(), "dir", ws.Dir, "branch", ws.Branch)
			return srv.Run()
		},
	}
	cmd.Flags().StringVarP(&agentName, "agent", "a", "auto", "chat agent (auto, claude, codex, echo)")
	cmd.Flags().IntVarP(&port, "port", "p", 8888, "HTTP listen port")
	return cmd
}

func toolShell(tasks *todo.Service, chats *chat.Service) ui.ToolShellProps {
	props := ui.ToolShellProps{}
	if chats != nil {
		props.ChatOptions = chat.ProviderModelOptions(chats.AgentNames())
	}
	if tasks != nil {
		for _, task := range shellTasks(tasks) {
			if !task.Archived {
				props.Counts.Inbox++
			}
			if task.Starred && !task.Archived {
				props.Counts.Starred++
			}
			props.Counts.Tasks++
		}
	}
	if chats != nil {
		for _, thread := range shellThreads(chats) {
			if thread.State != db.ThreadStateArchived {
				props.Counts.Inbox++
			}
			if thread.Starred && thread.State != db.ThreadStateArchived {
				props.Counts.Starred++
			}
			if chats.ThreadWorkflowID(thread) != "" {
				props.Counts.Workflows++
			} else {
				props.Counts.Chats++
			}
		}
	}
	return props
}

func shellTasks(tasks *todo.Service) []db.Task {
	items, err := tasks.All()
	if err != nil {
		return nil
	}
	return items
}

func shellThreads(chats *chat.Service) []db.Thread {
	items, err := chats.Threads()
	if err != nil {
		return nil
	}
	return items
}
