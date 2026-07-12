package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/cockroachdb/errors"
	"github.com/google/jsonschema-go/jsonschema"
	"github.com/housecat-inc/scratch/pkg/db"
	"github.com/housecat-inc/scratch/pkg/elicit"
	"github.com/housecat-inc/scratch/pkg/server/httperr"
	"github.com/housecat-inc/scratch/pkg/server/logging"
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
	mux.HandleFunc("POST /chat/{id}/elicitations", s.handleElicitation)
	mux.HandleFunc("GET /chat/{id}/events", s.handleEvents)
	mux.HandleFunc("POST /chat/{id}/messages", s.handleSend)
	return logging.Middleware(s.log, mux)
}

func (s *Server) handleElicitation(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	messageID, err := strconv.ParseInt(r.PostForm.Get("message_id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid message id", http.StatusBadRequest)
		return
	}
	msg, err := s.svc.store.GetMessage(messageID)
	if err != nil || msg.ThreadID != id {
		http.Error(w, "message not found", http.StatusNotFound)
		return
	}
	values := map[string]string{}
	for key := range r.PostForm {
		if name, ok := strings.CutPrefix(key, "f_"); ok {
			values[name] = r.PostForm.Get(key)
		}
	}
	err = s.svc.ResolveElicitation(messageID, r.PostForm.Get("elicitation_id"), r.PostForm.Get("action"), values)
	switch {
	case err == nil:
		w.WriteHeader(http.StatusNoContent)
	case elicit.IsInvalid(err):
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
	case errors.Is(err, ErrElicitationResolved):
		http.Error(w, "already resolved", http.StatusConflict)
	case errors.Is(err, ErrElicitationNotFound):
		http.Error(w, "elicitation not found", http.StatusNotFound)
	default:
		s.fail(w, err)
	}
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

	updates, take, cancel := s.svc.Subscribe(id)
	defer cancel()

	ping := time.NewTicker(ssePingInterval)
	defer ping.Stop()

	if err := s.writeMessagesEvent(w, r, id); err != nil {
		return
	}
	flusher.Flush()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-ping.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		case <-updates:
			if err := s.writeUpdates(w, r, id, take()); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func (s *Server) writeUpdates(w http.ResponseWriter, r *http.Request, threadID int64, messageIDs []int64) error {
	view, err := s.svc.View(threadID)
	if err != nil {
		return err
	}
	props := s.toThreadProps(view)
	byID := map[int64]ui.ChatMessageProps{}
	for _, m := range props.Messages {
		byID[m.ID] = m
	}
	for _, messageID := range messageIDs {
		if messageID == StructuralUpdate {
			return writeSSE(w, r.Context(), "message", ui.ChatMessages(props))
		}
		if _, ok := byID[messageID]; !ok {
			return writeSSE(w, r.Context(), "message", ui.ChatMessages(props))
		}
	}
	for _, messageID := range messageIDs {
		event := "message-" + strconv.FormatInt(messageID, 10)
		if err := writeSSE(w, r.Context(), event, ui.ChatMessage(byID[messageID])); err != nil {
			return err
		}
	}
	return nil
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

func (s *Server) fail(w http.ResponseWriter, err error) {
	httperr.Log(s.log, "chat error", err)
	httperr.Error(w, err, http.StatusInternalServerError)
}

func (s *Server) notFoundOr(w http.ResponseWriter, err error) {
	if db.IsThreadNotFound(err) {
		http.Error(w, "thread not found", http.StatusNotFound)
		return
	}
	s.fail(w, err)
}

func (s *Server) writeMessagesEvent(w http.ResponseWriter, r *http.Request, threadID int64) error {
	view, err := s.svc.View(threadID)
	if err != nil {
		return err
	}
	return writeSSE(w, r.Context(), "message", ui.ChatMessages(s.toThreadProps(view)))
}

func writeSSE(w http.ResponseWriter, ctx context.Context, event string, component templ.Component) error {
	var buf bytes.Buffer
	if err := component.Render(ctx, &buf); err != nil {
		return err
	}
	fmt.Fprintf(w, "event: %s\n", event)
	html := strings.ReplaceAll(buf.String(), "\r", "")
	for line := range strings.SplitSeq(html, "\n") {
		fmt.Fprintf(w, "data: %s\n", line)
	}
	_, err := fmt.Fprint(w, "\n")
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

func (s *Server) toThreadProps(v ThreadView) ui.ChatThreadProps {
	props := ui.ChatThreadProps{
		Agent:    s.svc.AgentName(v.Thread),
		ID:       v.Thread.ID,
		Messages: make([]ui.ChatMessageProps, 0, len(v.Messages)),
		Title:    v.Thread.Title,
	}
	if props.Title == "" {
		props.Title = "New chat"
	}
	for _, m := range v.Messages {
		props.Messages = append(props.Messages, MessageProps(v, m))
	}
	return props
}

func MessageProps(v ThreadView, m db.Message) ui.ChatMessageProps {
	msg := ui.ChatMessageProps{
		Author: m.Author,
		Body:   m.Body,
		ID:     m.ID,
		Parts:  toPartProps(v.Parts[m.ID]),
		Role:   m.Role,
		Status: m.Status,
	}
	if prompt, ok := v.Forms[m.ID]; ok {
		form := toFormProps(v.Thread.ID, m.ID, prompt)
		msg.Form = &form
	}
	return msg
}

func toPartProps(parts []MessagePart) []ui.ChatPartProps {
	props := make([]ui.ChatPartProps, 0, len(parts))
	for _, part := range parts {
		p := ui.ChatPartProps{Kind: part.Kind, Text: strings.TrimSpace(part.Text)}
		if (part.Kind == PartText || part.Kind == PartThinking) && p.Text == "" {
			continue
		}
		for _, entry := range part.Plan {
			p.Plan = append(p.Plan, ui.ChatPlanEntryProps{Content: entry.Content, Status: entry.Status})
		}
		if part.Tool != nil {
			p.Tool = &ui.ChatToolCallProps{
				Detail: part.Tool.Detail,
				ID:     part.Tool.ID,
				Status: part.Tool.Status,
				Title:  part.Tool.Title,
			}
		}
		props = append(props, p)
	}
	return props
}

func toFormProps(threadID, messageID int64, prompt elicit.Prompt) ui.ChatFormProps {
	form := ui.ChatFormProps{
		Action:        fmt.Sprintf("/chat/%d/elicitations", threadID),
		ElicitationID: prompt.ElicitationID,
		Message:       prompt.Message,
		MessageID:     messageID,
	}
	schema := prompt.RequestedSchema
	if schema == nil {
		return form
	}
	required := map[string]bool{}
	for _, name := range schema.Required {
		required[name] = true
	}
	for _, name := range prompt.Order {
		prop, ok := schema.Properties[name]
		if !ok {
			continue
		}
		form.Fields = append(form.Fields, toFieldProps(name, prop, required[name]))
	}
	return form
}

func toFieldProps(name string, prop *jsonschema.Schema, required bool) ui.ChatFormFieldProps {
	field := ui.ChatFormFieldProps{
		Label:    prop.Title,
		Name:     name,
		Required: required,
		Type:     "text",
	}
	if field.Label == "" {
		field.Label = name
	}
	switch prop.Type {
	case "boolean":
		field.Type = "checkbox"
	case "integer", "number":
		field.Type = "number"
	}
	if prop.Format == "email" {
		field.Type = "email"
	}
	for _, option := range prop.Enum {
		if s, ok := option.(string); ok {
			field.Options = append(field.Options, s)
		}
	}
	if len(field.Options) > 0 {
		field.Type = "select"
	}
	if len(prop.Default) > 0 {
		var s string
		if err := json.Unmarshal(prop.Default, &s); err == nil {
			field.Value = s
		} else {
			field.Value = string(prop.Default)
		}
	}
	return field
}
