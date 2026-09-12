package updates

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"sync"
	"time"
)

type Server struct {
	instance   string
	mu         sync.Mutex
	path       string
	restart    func() error
	restarting bool
	running    os.FileInfo
}

type status struct {
	Instance   string `json:"instance"`
	Pending    bool   `json:"pending"`
	Restarting bool   `json:"restarting"`
}

func New(service string) (*Server, error) {
	path, err := os.Executable()
	if err != nil {
		return nil, err
	}
	running, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	s := &Server{instance: strconv.FormatInt(time.Now().UnixNano(), 10), path: path, running: running}
	if service != "" {
		s.restart = func() error {
			return exec.Command("systemctl", "--user", "--no-block", "restart", service).Run()
		}
	}
	return s, nil
}

func (s *Server) pending() bool {
	installed, err := os.Stat(s.path)
	return s.restart != nil && err == nil && !os.SameFile(s.running, installed)
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /app/updates", s.handleStatus)
	mux.HandleFunc("POST /app/restart", s.handleRestart)
	return http.NewCrossOriginProtection().Handler(mux)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(status{Instance: s.instance, Pending: s.pending(), Restarting: s.restarting})
}

func (s *Server) handleRestart(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if r.Header.Get("X-Scratch-Instance") != s.instance {
		http.Error(w, "Refresh the page before restarting.", http.StatusForbidden)
		return
	}
	if s.restarting {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	if !s.pending() {
		http.Error(w, "No pending changes.", http.StatusConflict)
		return
	}
	if err := s.restart(); err != nil {
		http.Error(w, "Could not restart the app. Try again.", http.StatusInternalServerError)
		return
	}
	s.restarting = true
	w.WriteHeader(http.StatusAccepted)
}
