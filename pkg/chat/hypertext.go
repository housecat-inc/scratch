package chat

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/ui"
)

const ssePingInterval = 30 * time.Second

type Server struct {
	log *slog.Logger
	svc *Service
}

func NewServer(svc *Service, log *slog.Logger) *Server {
	if log == nil {
		log = slog.Default()
	}
	return &Server{log: log, svc: svc}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.Handle("GET /static/", http.StripPrefix("/static/", ui.StaticHandler()))
	mux.HandleFunc("GET /chat", s.handleIndex)
	mux.HandleFunc("GET /chat/{$}", s.handleIndex)
	mux.HandleFunc("POST /chat", s.handleCreate)
	mux.HandleFunc("GET /chat/{id}", s.handleThread)
	mux.HandleFunc("GET /chat/{id}/events", s.handleEvents)
	mux.HandleFunc("POST /chat/{id}/messages", s.handleSend)
	return s.logging(mux)
}

func (s *Server) handleCreate(w http.ResponseWriter, r *http.Request) {
	thread, err := s.svc.CreateThread(r.FormValue("title"))
	if err != nil {
		s.fail(w, err)
		return
	}
	http.Redirect(w, r, fmt.Sprintf("/chat/%d", thread.ID), http.StatusSeeOther)
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	if _, err := s.svc.Thread(id); err != nil {
		s.notFoundOr(w, err)
		return
	}

	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Content-Type", "text/event-stream")

	updates, cancel := s.svc.Subscribe(id)
	defer cancel()

	ping := time.NewTicker(ssePingInterval)
	defer ping.Stop()

	for {
		if err := s.writeMessagesEvent(w, r, id); err != nil {
			return
		}
		flusher.Flush()
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-updates:
		}
	}
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	threads, err := s.svc.Threads()
	if err != nil {
		s.fail(w, err)
		return
	}
	s.render(w, r, ui.ChatIndexPage(ui.ChatIndexProps{Threads: threads}))
}

func (s *Server) handleSend(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, err := s.svc.Send(id, prompt); err != nil {
		if IsThreadBusy(err) {
			http.Error(w, "agent is still responding", http.StatusConflict)
			return
		}
		s.notFoundOr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleThread(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	view, err := s.svc.View(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	s.render(w, r, ui.ChatThreadPage(toThreadProps(view)))
}

func (s *Server) fail(w http.ResponseWriter, err error) {
	s.log.Error("chat error", "error", err.Error())
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (s *Server) notFoundOr(w http.ResponseWriter, err error) {
	if db.IsThreadNotFound(err) {
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}
	s.fail(w, err)
}

func (s *Server) render(w http.ResponseWriter, r *http.Request, comp templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := comp.Render(r.Context(), w); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (s *Server) writeMessagesEvent(w http.ResponseWriter, r *http.Request, threadID int64) error {
	view, err := s.svc.View(threadID)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := ui.ChatMessages(toThreadProps(view)).Render(r.Context(), &buf); err != nil {
		return err
	}
	fmt.Fprint(w, "event: message\n")
	for line := range strings.SplitSeq(buf.String(), "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	_, err = fmt.Fprint(w, "\n")
	return err
}

func threadID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid thread id", http.StatusBadRequest)
		return 0, false
	}
	return id, true
}

func toThreadProps(v ThreadView) ui.ChatThreadProps {
	props := ui.ChatThreadProps{
		ID:       v.Thread.ID,
		Messages: make([]ui.ChatMessageProps, 0, len(v.Messages)),
		Title:    v.Thread.Title,
	}
	if props.Title == "" {
		props.Title = "New chat"
	}
	for _, m := range v.Messages {
		props.Messages = append(props.Messages, ui.ChatMessageProps{
			Author: m.Author,
			Body:   m.Body,
			ID:     m.ID,
			Role:   m.Role,
			Status: m.Status,
			Tools:  v.ToolCalls[m.ID],
		})
	}
	return props
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (s *Server) logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		s.log.Info("http.request",
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
		)
	})
}
