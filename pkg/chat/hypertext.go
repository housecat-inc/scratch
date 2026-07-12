package chat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"mime"
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
	mux.HandleFunc("GET /chat/popout/new", s.handleNewPopout)
	mux.HandleFunc("GET /chat/{id}", http.NotFound)
	mux.HandleFunc("DELETE /chat/{id}", s.handleDelete)
	mux.HandleFunc("POST /chat/{id}/access", s.handleAccess)
	mux.HandleFunc("POST /chat/{id}/attachments", s.handleUpload)
	mux.HandleFunc("GET /chat/{id}/attachments/{attachment}", s.handleAttachment)
	mux.HandleFunc("DELETE /chat/{id}/attachments/{attachment}", s.handleDeleteAttachment)
	mux.HandleFunc("POST /chat/{id}/elicitations", s.handleElicitation)
	mux.HandleFunc("GET /chat/{id}/events", s.handleEvents)
	mux.HandleFunc("POST /chat/{id}/messages", s.handleSend)
	mux.HandleFunc("GET /chat/{id}/popout", s.handlePopout)
	mux.HandleFunc("POST /chat/{id}/stop", s.handleStop)
	return logging.Middleware(s.log, mux)
}

func (s *Server) handleNewPopout(w http.ResponseWriter, r *http.Request) {
	agent := strings.TrimSpace(r.URL.Query().Get("agent"))
	model := strings.TrimSpace(r.URL.Query().Get("model"))
	if providerModel := r.URL.Query().Get("provider_model"); providerModel != "" {
		agent, model = ParseProviderModel(providerModel)
	}
	label := strings.TrimSpace(r.URL.Query().Get("chat_label"))
	if model == "default" {
		model = ""
	}
	thread, err := s.svc.CreateThreadWithLabel(agent, model, label, "")
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if err := s.renderPopout(w, r, thread.ID); err != nil {
		s.fail(w, err)
	}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	if err := s.svc.DeleteThread(id); err != nil {
		s.notFoundOr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePopout(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	if err := s.renderPopout(w, r, id); err != nil {
		s.notFoundOr(w, err)
	}
}

func (s *Server) renderPopout(w http.ResponseWriter, r *http.Request, threadID int64) error {
	view, err := s.svc.View(threadID)
	if err != nil {
		return err
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	return ui.FloatingChatPanel(s.toFloatingProps(view)).Render(r.Context(), w)
}

func (s *Server) handleAccess(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	access, err := s.svc.ToggleThreadAccess(id)
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if err := ui.ChatAccessToggle(id, access).Render(r.Context(), w); err != nil {
		s.fail(w, err)
	}
}

func (s *Server) handleAttachment(w http.ResponseWriter, r *http.Request) {
	attachment, ok := s.threadAttachment(w, r)
	if !ok {
		return
	}
	w.Header().Set("Content-Type", attachment.MimeType)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("inline", map[string]string{"filename": attachment.Name}))
	http.ServeFile(w, r, attachment.FilePath)
}

func (s *Server) handleDeleteAttachment(w http.ResponseWriter, r *http.Request) {
	attachment, ok := s.threadAttachment(w, r)
	if !ok {
		return
	}
	err := s.svc.DeleteDraftAttachment(attachment.ID)
	if db.IsAttachmentNotFound(err) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return
	}
	if err != nil {
		s.fail(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) threadAttachment(w http.ResponseWriter, r *http.Request) (db.Attachment, bool) {
	threadID, ok := threadID(w, r)
	if !ok {
		return db.Attachment{}, false
	}
	id, err := strconv.ParseInt(r.PathValue("attachment"), 10, 64)
	if err != nil {
		http.Error(w, "invalid attachment id", http.StatusBadRequest)
		return db.Attachment{}, false
	}
	attachment, err := s.svc.Attachment(id)
	if db.IsAttachmentNotFound(err) || (err == nil && attachment.ThreadID != threadID) {
		http.Error(w, "attachment not found", http.StatusNotFound)
		return db.Attachment{}, false
	}
	if err != nil {
		s.fail(w, err)
		return db.Attachment{}, false
	}
	return attachment, true
}

func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, attachmentMaxBytes+(1<<20))
	file, header, err := r.FormFile("file")
	if err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			http.Error(w, ErrAttachmentTooLarge.Error(), http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "missing file", http.StatusBadRequest)
		return
	}
	defer file.Close()
	attachment, err := s.svc.AddAttachment(id, header.Filename, header.Header.Get("Content-Type"), file)
	if errors.Is(err, ErrAttachmentTooLarge) {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	if err != nil {
		s.notFoundOr(w, err)
		return
	}
	if err := ui.ChatAttachmentChip(toAttachmentProps(attachment)).Render(r.Context(), w); err != nil {
		s.fail(w, err)
	}
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
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	attachmentIDs := []int64{}
	for _, raw := range r.Form["attachment_id"] {
		aid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			http.Error(w, "invalid attachment id", http.StatusBadRequest)
			return
		}
		attachmentIDs = append(attachmentIDs, aid)
	}
	prompt := strings.TrimSpace(r.FormValue("prompt"))
	if prompt == "" {
		if len(attachmentIDs) == 0 {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		prompt = "See attached files."
	}
	if _, err := s.svc.Send(id, prompt, attachmentIDs...); err != nil {
		if IsThreadBusy(err) {
			http.Error(w, "agent is still responding", http.StatusConflict)
			return
		}
		s.notFoundOr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleStop(w http.ResponseWriter, r *http.Request) {
	id, ok := threadID(w, r)
	if !ok {
		return
	}
	if err := s.svc.StopThread(id); err != nil && !IsThreadNotBusy(err) {
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
	if db.IsAttachmentNotFound(err) {
		http.Error(w, "attachment not found", http.StatusNotFound)
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

func (s *Server) toFloatingProps(v ThreadView) ui.FloatingChatProps {
	thread := s.toThreadProps(v)
	return ui.FloatingChatProps{
		Access:      s.svc.ThreadAccess(v.Thread),
		Agent:       thread.Agent,
		Description: FriendlyDescription(threadPromptFromMessages(v.Messages)),
		ID:          thread.ID,
		Messages:    thread.Messages,
		Streaming:   v.Streaming,
		Title:       thread.Title,
	}
}

func threadPromptFromMessages(messages []db.Message) string {
	for _, m := range messages {
		if m.Role == db.MessageRoleUser {
			return m.Body
		}
	}
	return ""
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
	for _, a := range v.Attachments[m.ID] {
		msg.Attachments = append(msg.Attachments, toAttachmentProps(a))
	}
	if prompt, ok := v.Forms[m.ID]; ok {
		form := toFormProps(v.Thread.ID, m.ID, prompt)
		msg.Form = &form
	}
	return msg
}

func toAttachmentProps(a db.Attachment) ui.ChatAttachmentProps {
	return ui.ChatAttachmentProps{
		ID:       a.ID,
		IsImage:  strings.HasPrefix(a.MimeType, "image/"),
		Name:     a.Name,
		ThreadID: a.ThreadID,
	}
}

func toPartProps(parts []MessagePart) []ui.ChatPartProps {
	props := make([]ui.ChatPartProps, 0, len(parts))
	for _, part := range parts {
		p := ui.ChatPartProps{
			Kind:    part.Kind,
			Summary: summaryLine(part.Text),
			Text:    strings.TrimSpace(part.Text),
		}
		if (part.Kind == PartText || part.Kind == PartThinking) && p.Text == "" {
			continue
		}
		for _, entry := range part.Plan {
			p.Plan = append(p.Plan, ui.ChatPlanEntryProps{Content: entry.Content, Status: entry.Status})
		}
		if part.Tool != nil {
			p.Tool = &ui.ChatToolCallProps{
				Detail:  part.Tool.Detail,
				ID:      part.Tool.ID,
				Status:  part.Tool.Status,
				Summary: summaryLine(part.Tool.Detail),
				Title:   part.Tool.Title,
			}
		}
		props = append(props, p)
	}
	return props
}

func summaryLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return clip(strings.TrimSpace(line), 140)
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
